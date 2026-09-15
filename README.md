# AtlasGo GoMVC SDK

Go + Gin SDK for Atlas-style WebMVC services. It mirrors the Java `atlas-sdk-webmvc` baseline with a Go module layout:

- Common DTOs and errors in one `common` package: `common.Response[T]`, `common.Paged[T]`, and `common.Error`.
- Extensible request context: `web.Context`, `web.ContextLoader`, `web.ResolveToken` and the JWT crypto (`auth.JWTCodec`); the same `*web.Context` is readable over HTTP, gRPC and WebSocket.
- Gin Web support: WebContext extraction, `ContextCustomizer` extension points, route-level `RequireAuth`, required-by-default auth, SDK-level CORS, response wrapping, and unified error responses.
- Optional integrations as separate modules: MongoDB with cluster/datastore/entity routing and auto-index, Redis with YAML pool mapping, and gRPC with multiple named service channels.
- Local demo project with one combined `config.yaml` for SDK validation and integration examples.

The SDK source is hosted at [github.com/ISeekFree/AtlasGo](https://github.com/ISeekFree/AtlasGo). After the initial `v0.1.0` release tags are pushed, consumers can install the core SDK or only the optional integration modules they need:

```bash
go get github.com/ISeekFree/AtlasGo@latest
go get github.com/ISeekFree/AtlasGo/integrations/mongo@latest
go get github.com/ISeekFree/AtlasGo/integrations/redis@latest
go get github.com/ISeekFree/AtlasGo/integrations/grpc@latest
```

The repository uses nested Go modules. For a release, tag the root module as `v0.1.0` and each optional module with its directory prefix (`integrations/mongo/v0.1.0`, `integrations/redis/v0.1.0`, and `integrations/grpc/v0.1.0`). The gRPC module requires the matching root tag.

## Modules

| Path | Go module | Role |
| --- | --- | --- |
| `.` | `github.com/ISeekFree/AtlasGo` | Core SDK: common DTOs/errors, auth, and Gin middleware. |
| `integrations/mongo` | `github.com/ISeekFree/AtlasGo/integrations/mongo` | MongoDB config loader, registry, entity routing/indexes, and generic CRUD. |
| `integrations/redis` | `github.com/ISeekFree/AtlasGo/integrations/redis` | Redis config loader, client options mapping, and key builder. |
| `integrations/grpc` | `github.com/ISeekFree/AtlasGo/integrations/grpc` | gRPC config loader, named channels, metadata keys, and identity context helpers. Auth interceptors are business-owned. |
| `demo` | `github.com/ISeekFree/AtlasGo/demo` | Local validation app and integration guide. |

## Integration And Configuration Model

The optional modules remain independent. An integrating project can use any subset and can choose YAML, explicit Go options, or a mixture of both:

| Integration | YAML namespace and loader | Explicit construction remains available |
| --- | --- | --- |
| MongoDB | `framework.mongo` via `mongo.LoadConfig` | `mongo.Connect(ctx, mongo.Options{...})` |
| Redis | `framework.redis` via `redis.LoadConfig` | `redis.NewClient(redis.Options{...})` |
| gRPC | `framework.grpc` via `atlasgrpc.LoadConfig` | `atlasgrpc.NewChannelFactory(atlasgrpc.ClientOptions{...})` and standard `grpc.NewServer` |

All loaders accept a complete project YAML document, select their own namespace, and expand `${ENV:default}` placeholders. Loading configuration does not couple the optional modules. The demo composes all three from [demo/config.yaml](demo/config.yaml).

## Verify

```bash
./build.sh
```

## Minimal Gin Usage

```go
engine := gin.New()
sdk := web.New()
sdk.Install(engine)

engine.GET("/demo/me", sdk.RequireAuth(web.Domains("app.demo"), web.Permissions("demo:read")), handler)
```

Tokens are JWT. AtlasGo ships the `auth.JWTCodec` crypto but no claim schema: register a `web.ContextLoader` through `web.Options.ContextLoaders` that verifies the token with `auth.JWTCodec` and fills `*web.Context`. Permission checks for `RequireAuth(web.Permissions(...))` go to an optional `web.Options.PermissionChecker`.

## Response Envelope

Successful JSON/text responses are wrapped into `{code:0,msg:"success",data:...}`. Failures use the same shape: authentication failures return HTTP 200 with `common.UnauthorizedCode` (-94), and a recovered panic or a `c.Error(...)` report returns HTTP 200 with `common.SystemErrorCode` (-90), unless the failure carries its own business code. Build the unified error with `common.NewError("msg")` (defaults to -90) or `common.NewErrorCode(code, "msg")` for an explicit code; `common.WrapError` / `common.WrapErrorCode` do the same with a cause. Register `web.Options.ErrorResolvers` to map failures onto your own envelope.

## WebContext Extension

Business services can add request-scoped attributes before auth runs:

```go
sdk := web.New(web.Options{
	ContextCustomizers: []web.ContextCustomizer{
		func(wc *web.Context, c *gin.Context) {
			wc.SetAttribute("traceId", c.GetHeader("x-trace-id"))
		},
	},
})
```

Custom attributes are available through `web.Context` and are copied into `auth.Request.Attributes`.

## MongoDB Configuration And Entities

`mongo.LoadConfig` reads the `framework.mongo` section from the integrating project's YAML file and resolves `${ENV:default}` placeholders. Both flat datastores and the Java-compatible `cluster -> datastores` layout are supported:

```yaml
framework:
  mongo:
    auto-index: true
    clusters:
      primary:
        uri: ${MONGO_URI:mongodb://127.0.0.1:27017}
        datastores:
          catalog:
            database: catalog
          audit:
            database: audit
```

Register each entity explicitly because Go cannot scan packages like Morphia. An entity can declare its datastore and collection. Simple indexes can use `mongo` field tags; compound indexes use an entity-level `MongoIndexes` declaration, equivalent to Morphia's `@Indexes`:

```go
type Product struct {
	TenantID string    `bson:"tenantId"`
	SKU      string    `bson:"sku"`
	Updated  time.Time `bson:"updated" mongo:"index,order=-1"`
}

func (Product) MongoDatastore() string { return "catalog" }
func (Product) MongoCollection() string { return "products" }
func (Product) MongoIndexes() []mongo.IndexDefinition {
	return []mongo.IndexDefinition{
		mongo.CompoundIndex(
			"uk_tenant_sku",
			mongo.Asc("tenantId"),
			mongo.Asc("sku"),
		).WithUnique(),
	}
}

config, err := mongo.LoadConfig("config.yaml")
config.Entities = []mongo.EntityMapping{{Model: Product{}}}
registry, err := mongo.Connect(ctx, config)
products, err := registry.CollectionFor(Product{})
```

When effective `auto-index` is enabled, `Connect` scans registered entities and creates their indexes idempotently. It can be overridden globally, per cluster, or per datastore. `CompoundIndex` preserves key order and each `Asc`/`Desc` direction. Fields with the same explicit tag index name are also combined for compatibility. Supported tag options are `index[=name]`, `unique`, `sparse`, `hidden`, `order=1|-1|text|hashed|2d|2dsphere`, and `expire=<seconds>`.

The existing programmatic multi-cluster routing remains available:

```go
registry, err := mongo.Connect(ctx, mongo.Options{
	Clusters: map[string]mongo.ClusterOptions{
		"orders": {URI: "mongodb://orders-cluster"},
	},
	Datastores: map[string]mongo.DatastoreOptions{
		"order-read": {Cluster: "orders", Database: "orders"},
	},
})
collection := registry.Collection("orders", "order-read")
```

## Redis Configuration

`redis.LoadConfig("config.yaml")` reads `framework.redis`, including host, port, database, credentials, timeouts, key prefix, and pool settings. `${ENV:default}` placeholders use the same behavior as MongoDB:

```go
config, err := redis.LoadConfig("config.yaml")
if config.IsEnabled() {
	client := redis.NewClient(config)
	keys := redis.NewKeyBuilder(config.KeyPrefix)
}
```

## gRPC Configuration And Named Services

`atlasgrpc.LoadConfig("config.yaml")` reads the server listener and any number of named client channels. Each service group has its own target and connection settings:

```go
config, err := atlasgrpc.LoadConfig("config.yaml")
listener, err := net.Listen("tcp", config.Server.ListenAddress())
factory := atlasgrpc.NewChannelFactory(config.Client)

accountConn, err := factory.Channel(ctx, "account-service")
orderConn, err := factory.Channel(ctx, "order-service")
```

Configuration is optional. Channels can still be created entirely in code:

```go
factory := atlasgrpc.NewChannelFactory(atlasgrpc.ClientOptions{
	Channels: map[string]atlasgrpc.ChannelOptions{
		"account-service": {Target: "static://account.internal:19091", Plaintext: true},
	},
	UnaryInterceptors: []grpc.UnaryClientInterceptor{yourTokenPropagationInterceptor},
})
```

AtlasGo does not install an auth interceptor: which HTTP header carries the caller token and which metadata key the callee expects are business contracts. Register your own `grpc.UnaryClientInterceptor`/`grpc.StreamClientInterceptor` through `ClientOptions`, and your own `grpc.UnaryServerInterceptor`/`grpc.StreamServerInterceptor` through `grpc.NewServer(...)`; AtlasGo provides the `Metadata*` keys plus `atlasgrpc.MetadataRequest`, `atlasgrpc.WithContext` and `atlasgrpc.ContextFromContext` so the server can reuse the same `web.ContextLoader` as HTTP.

Targets follow gRPC naming syntax. AtlasGo registers `static://host:port` for a fixed single address and makes `static` the default resolver, so a target without a scheme, such as `127.0.0.1:19091` or `account.internal:19091`, is also treated as static. IPv4, domain names, and bracketed IPv6 are supported. The grpc-go built-ins `dns:///service:port`, `unix:///path/to.sock`, and `passthrough:///service` are available by default. AtlasGo also registers the official `xds:///service-name` resolver by default; using it requires an xDS runtime/bootstrap configuration. `${:default}` placeholders are accepted when a config value should always fall back to the default string.

Named channels are lazy: defining `account-service` or `order-service` does not establish a connection until `Channel(ctx, name)` is called. The channel name is the stable service-group routing key; targets and connection settings may differ for every group.

The demo includes a real protobuf contract at [demo/proto/demo/v1/demo.proto](demo/proto/demo/v1/demo.proto). Its generated server/client exposes two authenticated unary RPCs:

- `Echo` demonstrates request/response messages and the user ID forwarded by the demo's business-layer client interceptor.
- `CurrentUser` returns the identity resolved by the demo's business-layer gRPC auth interceptor.

Generated Go files are committed under `demo/gen`; the demo no longer maintains a hand-written `grpc.ServiceDesc`.
