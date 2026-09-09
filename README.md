# claw-sdk-gomvc

Go + Gin SDK for Claw-style WebMVC services. It mirrors the Java `claw-sdk-webmvc` baseline with a Go module layout:

- Common DTOs and errors in one `common` package: `common.Response[T]`, `common.Paged[T]`, and `common.Error`.
- Unified auth contracts: `auth.Service`, `auth.Request`, `auth.Identity`, and demo-friendly `auth.CookieStyleService`.
- Gin Web support: WebContext extraction, `ContextCustomizer` extension points, route-level `RequireAuth`, required-by-default auth, response wrapping, and unified error responses.
- Optional integrations as separate modules: MongoDB with cluster/datastore/entity routing and auto-index, Redis with YAML pool mapping, and gRPC with multiple named service channels.
- Local demo project with one combined `config.yaml` for SDK validation and integration examples.

The SDK is intentionally local-module friendly. It does not require publishing to GitHub or any registry; consumers can use `replace` directives.

## Modules

| Path | Go module | Role |
| --- | --- | --- |
| `.` | `iseekfree.com/common/sdk/gomvc` | Core SDK: common DTOs/errors, auth, and Gin middleware. |
| `integrations/mongo` | `iseekfree.com/common/sdk/gomvc/mongo` | MongoDB config loader, registry, entity routing/indexes, and generic CRUD. |
| `integrations/redis` | `iseekfree.com/common/sdk/gomvc/redis` | Redis config loader, client options mapping, and key builder. |
| `integrations/grpc` | `iseekfree.com/common/sdk/gomvc/grpc` | gRPC config loader, named channels, auth context, and interceptors. |
| `demo` | `iseekfree.com/common/sdk/gomvc/demo` | Local validation app and integration guide. |

## Integration And Configuration Model

The optional modules remain independent. An integrating project can use any subset and can choose YAML, explicit Go options, or a mixture of both:

| Integration | YAML namespace and loader | Explicit construction remains available |
| --- | --- | --- |
| MongoDB | `claw.mongo` via `mongo.LoadConfig` | `mongo.Connect(ctx, mongo.Options{...})` |
| Redis | `claw.redis` via `redis.LoadConfig` | `redis.NewClient(redis.Options{...})` |
| gRPC | `claw.grpc` via `clawgrpc.LoadConfig` | `clawgrpc.NewChannelFactory(clawgrpc.ClientOptions{...})` and standard `grpc.NewServer` |

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

Override `auth.Service` in `web.Options` for production identity/session validation.

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

`mongo.LoadConfig` reads the `claw.mongo` section from the integrating project's YAML file and resolves `${ENV:default}` placeholders. Both flat datastores and the Java-compatible `cluster -> datastores` layout are supported:

```yaml
claw:
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

`redis.LoadConfig("config.yaml")` reads `claw.redis`, including host, port, database, credentials, timeouts, key prefix, and pool settings. `${ENV:default}` placeholders use the same behavior as MongoDB:

```go
config, err := redis.LoadConfig("config.yaml")
if config.IsEnabled() {
	client := redis.NewClient(config)
	keys := redis.NewKeyBuilder(config.KeyPrefix)
}
```

## gRPC Configuration And Named Services

`clawgrpc.LoadConfig("config.yaml")` reads the server listener and any number of named client channels. Each service group has its own target and connection settings:

```go
config, err := clawgrpc.LoadConfig("config.yaml")
listener, err := net.Listen("tcp", config.Server.ListenAddress())
factory := clawgrpc.NewChannelFactory(config.Client)

accountConn, err := factory.Channel(ctx, "account-service")
orderConn, err := factory.Channel(ctx, "order-service")
```

Configuration is optional. Channels can still be created entirely in code:

```go
factory := clawgrpc.NewChannelFactory(clawgrpc.ClientOptions{
	Channels: map[string]clawgrpc.ChannelOptions{
		"account-service": {Target: "account.internal:19091", Plaintext: true},
	},
})
```

Named channels are lazy: defining `account-service` or `order-service` does not establish a connection until `Channel(ctx, name)` is called. The channel name is the stable service-group routing key; targets and connection settings may differ for every group.

The demo includes a real protobuf contract at [demo/proto/demo/v1/demo.proto](demo/proto/demo/v1/demo.proto). Its generated server/client exposes two authenticated unary RPCs:

- `Echo` demonstrates request/response messages and propagated user identity.
- `CurrentUser` returns the identity resolved by the gRPC server auth interceptor.

Generated Go files are committed under `demo/gen`; the demo no longer maintains a hand-written `grpc.ServiceDesc`.
