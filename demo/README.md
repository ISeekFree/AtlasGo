# AtlasGo demo

This demo validates the SDK using the public `github.com/ISeekFree/AtlasGo` module paths. Its `go.mod` uses local `replace` directives so contributors always exercise the current checkout; consumers should install the GitHub modules directly.

The examples use `@latest`; for the first release, publish the root `v0.1.0` tag and the prefixed integration tags (`integrations/mongo/v0.1.0`, `integrations/redis/v0.1.0`, and `integrations/grpc/v0.1.0`) before running these commands.

## Run

```bash
go run .
```

The demo loads MongoDB, Redis, and gRPC settings from [config.yaml](config.yaml). Set `ATLAS_DEMO_CONFIG` to use another file. MongoDB and Redis are disabled by default and can be enabled through `ATLAS_MONGO_ENABLED=true` and `ATLAS_REDIS_ENABLED=true`.

Initialization follows one visible path: `configFromFile` invokes each SDK loader, `newDemoApp` starts the configured gRPC server and named channel factory, and `setupOptionalClients` initializes only enabled MongoDB/Redis integrations.

Default endpoints:

- `GET /demo/public`
- `GET /demo/me`
- `GET /demo/grpc`
- `GET /demo/grpc/echo?message=hello`
- `GET /demo/grpc/me`
- `GET /demo/stream`
- `GET /demo/options`
- `GET /demo/redis/basic` when Redis config is present
- `GET /demo/mongo/basic` when MongoDB config is present

Use a demo token:

```text
_u_=u1;_d_=app.demo;_perms_=demo:read
```

## Web-only integration

In another project:

```bash
go get github.com/ISeekFree/AtlasGo@latest
```

Then install the Gin middleware:

```go
engine := gin.New()
sdk := web.New()
sdk.Install(engine)
engine.GET("/me", sdk.RequireAuth(web.Domains("app.demo")), handler)
```

Override `auth.Service` in `web.Options` to connect the SDK to the real account/session system.

To add business fields to `WebContext`:

```go
sdk := web.New(web.Options{
	ContextCustomizers: []web.ContextCustomizer{
		func(wc *web.Context, c *gin.Context) {
			wc.SetAttribute("demoTraceId", c.GetHeader("x-demo-trace-id"))
		},
	},
})
```

## Optional MongoDB integration

Add only when the project needs MongoDB:

```bash
go get github.com/ISeekFree/AtlasGo/integrations/mongo@latest
```

Use `mongo.LoadConfig("config.yaml")` to read the integrating project's `framework.mongo` section; see [config.yaml](config.yaml) for a single-cluster, multi-database example. Register entities in Go, then keep the returned registry in your application container:

```go
type Product struct {
	TenantID string `bson:"tenantId"`
	SKU      string `bson:"sku"`
}

func (Product) MongoDatastore() string { return "catalog" }
func (Product) MongoCollection() string { return "products" }
func (Product) MongoIndexes() []mongo.IndexDefinition {
	return []mongo.IndexDefinition{
		mongo.CompoundIndex("uk_product_tenant_sku", mongo.Asc("tenantId"), mongo.Asc("sku")).WithUnique(),
	}
}

config, err := mongo.LoadConfig("config.yaml")
config.Entities = []mongo.EntityMapping{{Model: Product{}}}
registry, err := mongo.Connect(ctx, config)
collection, err := registry.CollectionFor(Product{})
```

For multiple clusters, the existing programmatic routing also remains available:

```go
registry, err := mongo.Connect(ctx, mongo.Options{
	Clusters: map[string]mongo.ClusterOptions{
		"finance": {URI: "mongodb://finance-cluster"},
		"orders":  {URI: "mongodb://orders-cluster"},
	},
	Datastores: map[string]mongo.DatastoreOptions{
		"finance-main": {Cluster: "finance", Database: "finance"},
		"orders-main":  {Cluster: "orders", Database: "orders"},
	},
})
```

## Optional Redis integration

Add only when the project needs Redis:

```bash
go get github.com/ISeekFree/AtlasGo/integrations/redis@latest
```

Use `redis.LoadConfig("config.yaml")`, then pass the result to `redis.NewClient`. The `framework.redis` section supports host, port, database, credentials, key prefix, common timeouts, and Java-style pool settings:

```go
config, err := redis.LoadConfig("config.yaml")
if config.IsEnabled() {
	client := redis.NewClient(config)
	keys := redis.NewKeyBuilder(config.KeyPrefix)
}
```

## Optional gRPC integration

Add only when the project needs gRPC:

```bash
go get github.com/ISeekFree/AtlasGo/integrations/grpc@latest
```

`atlasgrpc.LoadConfig("config.yaml")` reads `framework.grpc.server` and all named entries under `framework.grpc.client.channels`. The demo defines `local`, `account-service`, and `order-service`; connections are created lazily when their names are requested:

```go
config, err := atlasgrpc.LoadConfig("config.yaml")
factory := atlasgrpc.NewChannelFactory(config.Client)
accountConn, err := factory.Channel(ctx, "account-service")
orderConn, err := factory.Channel(ctx, "order-service")
```

The `/demo/grpc` endpoint requests the `local` channel. Its target comes from `framework.grpc.client.channels.local.target`; the server listener comes independently from `framework.grpc.server.host/port`. When tests use server port `0`, the demo replaces only the local target with the actual allocated listener address.

Manual construction remains supported and is useful when targets come from service discovery or code:

```go
factory := atlasgrpc.NewChannelFactory(atlasgrpc.ClientOptions{
	Channels: map[string]atlasgrpc.ChannelOptions{
		"account-service": {Target: "account.internal:19091", Plaintext: true},
	},
})
```

Server side uses `atlasgrpc.UnaryServerAuthInterceptor`; client channels resolve the current HTTP token from the original Gin request headers and propagate it by default.

### Protobuf contract

The source contract is [proto/demo/v1/demo.proto](proto/demo/v1/demo.proto):

- `DemoService.Echo` returns the message and propagated user ID.
- `DemoService.CurrentUser` returns user ID, domain, and permissions from the gRPC auth context.

Generated message and gRPC code lives under `gen/demo/v1` and is used directly by both the demo server and client. To regenerate after editing the proto:

```bash
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
../protoc.sh
```

Do not edit generated `.pb.go` files or reintroduce hand-written `grpc.ServiceDesc` definitions.

## External Redis/Mongo Checks

Normal `go test ./...` skips external storage checks. To run the supplied Redis and MongoDB basic operation test:

```bash
ATLAS_DEMO_EXTERNAL_TEST=1 go test . -run TestDemoExternalRedisAndMongoBasicOperations -count=1
```

The test uses the supplied Redis host/database/pool settings and MongoDB database `atlas-sdk-demo` by default. Override with:

```bash
ATLAS_DEMO_MONGO_URI=... \
ATLAS_DEMO_MONGO_DATABASE=atlas-sdk-demo \
ATLAS_DEMO_REDIS_HOST=... \
ATLAS_DEMO_REDIS_PORT=6379 \
ATLAS_DEMO_REDIS_DATABASE=1 \
ATLAS_DEMO_REDIS_PASSWORD=... \
ATLAS_DEMO_EXTERNAL_TEST=1 go test . -run TestDemoExternalRedisAndMongoBasicOperations -count=1
```
