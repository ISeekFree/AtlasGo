# AtlasGo Agent Guide

This is the canonical coding-agent guide for the repository. Read [Architecture.md](Architecture.md) before making cross-module changes.

## Repository Shape

| Path | Go module | Responsibility |
| --- | --- | --- |
| `.` | `github.com/ISeekFree/AtlasGo` | `common`, auth contracts, Gin context/auth/response middleware. |
| `integrations/mongo` | `github.com/ISeekFree/AtlasGo/integrations/mongo` | MongoDB configuration, cluster/datastore registry, entity routing, indexes, and CRUD. |
| `integrations/redis` | `github.com/ISeekFree/AtlasGo/integrations/redis` | Redis configuration, client creation, pool mapping, and key prefixes. |
| `integrations/grpc` | `github.com/ISeekFree/AtlasGo/integrations/grpc` | gRPC configuration, named channels, auth propagation, and server/client interceptors. |
| `demo` | `github.com/ISeekFree/AtlasGo/demo` | Local integration example and validation only. |

## Non-Negotiable Boundaries

- Keep optional integrations as separate modules. The root module must not import MongoDB, Redis, or gRPC implementations.
- Keep shared errors and response/page DTOs in `common`; do not recreate standalone `atlaserr` or `response` packages.
- Preserve Java-compatible `{code,msg,data}` response semantics. Business and authentication failures use HTTP 200 with a non-zero business code.
- Extend request-specific business context through `web.ContextCustomizer` and `web.Context.Attributes`, not one-off core fields.
- Production authentication replaces `auth.Service`; Web and gRPC must continue depending on that interface.
- YAML loading is an optional convenience. Every integration must continue supporting explicit programmatic options and constructors.

## Configuration Contract

Each optional module independently reads its namespace from the integrating project's YAML file and expands `${ENV:default}` placeholders:

| Namespace | Loader | Runtime construction |
| --- | --- | --- |
| `framework.mongo` | `mongo.LoadConfig` | Append entity mappings, then call `mongo.Connect`. |
| `framework.redis` | `redis.LoadConfig` | Check `IsEnabled`, then call `redis.NewClient`. |
| `framework.grpc` | `atlasgrpc.LoadConfig` | Use `Server.ListenAddress` and `NewChannelFactory(config.Client)`. |

Do not couple the three loaders or make the optional modules depend on one another. The demo may compose all three from one `config.yaml`.

## MongoDB Rules

- Preserve both legacy flat `Options.Datastores` and nested `cluster -> datastores` configuration.
- Datastore names are globally unique and represent logical databases. Multiple entities may map to one datastore/database and different collections.
- Go cannot scan packages like Morphia. Consumers explicitly register `EntityMapping` values.
- Entity routing comes from `EntityMetadata` or explicit `MapEntity` values; explicit values win.
- Simple indexes may use `mongo` field tags. Compound indexes should use entity-level `MongoIndexes() []IndexDefinition`, mirroring Morphia `@Indexes`.
- Preserve compound key order and direction. `WithUnique` applies uniqueness to the whole compound index.
- `auto-index` inherits from global to cluster to datastore and only processes registered entities.
- Manual `EnsureIndexes` must remain available for driver-level index models.

## Redis Rules

- Preserve direct `redis.NewClient(redis.Options{...})` usage.
- `framework.redis` supports `addr` or `host`/`port`, database, credentials, key prefix, common timeouts, and Java-style pool fields.
- Map `connect-timeout` to dial timeout, `timeout` to read/write timeouts, `max-active` to pool size, and `max-wait` to pool timeout.
- Configuration parsing must not connect to Redis; connection remains lazy in go-redis.

## gRPC Rules

- `ClientOptions.Channels` is a map of named service groups. Multiple targets such as `account-service` and `order-service` must remain supported.
- Channels are created lazily by `Channel(ctx, name)` and cached by name.
- Preserve fully manual `NewChannelFactory(ClientOptions)` construction, including code-only interceptors.
- Preserve automatic WebContext token propagation unless explicitly disabled.
- Server listener/auth configuration is separate from client channels. Do not assume every client channel points to the local server.

## Demo And Verification

- `demo/config.yaml` is the canonical combined configuration example for MongoDB, Redis, and gRPC.
- MongoDB and Redis stay disabled by default so ordinary tests require no external services.
- Named gRPC channels other than `local` are lazy and may point at services that are not running during demo tests.
- `demo/proto` is the source of truth for demo gRPC contracts. Commit generated files under `demo/gen`; do not restore hand-written `grpc.ServiceDesc` stubs.
- When a proto changes, run `./protoc.sh` from the repository root, or `go generate ./...` from `demo`; both use the same generator path. Exercise every demo RPC through its generated client in tests.
- The demo uses local `replace` directives to exercise the current checkout; public examples use the GitHub module paths.
- External Redis/Mongo checks remain opt-in with `ATLAS_DEMO_EXTERNAL_TEST=1`.
- Run `./build.sh` before completing broad SDK changes.
