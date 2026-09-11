# AtlasGo GoMVC Architecture

## Baseline

- Runtime target: Go 1.23.
- Web framework: Gin.
- Module root: `github.com/ISeekFree/AtlasGo`.
- Optional integrations are separate Go modules under `integrations/` so consumers only add the dependencies they use.

## Modules

| Module | Role |
| --- | --- |
| `auth` | Shared auth contracts, `Identity`, `Request`, permission checks, and `CookieStyleService`. |
| `common` | SDK error type, common error codes, unified `Response<T>`, and `Paged<T>` payloads. |
| `web` | Gin middleware for WebContext, extension customizers, auth, response wrapping, and panic/error conversion. |
| `integrations/mongo` | MongoDB YAML loading, multi-cluster/datastore registry, entity routing/indexes, and CRUD. |
| `integrations/redis` | Redis YAML loading, client/pool option mapping, and key prefix helper. |
| `integrations/grpc` | gRPC YAML loading, named channels, metadata/auth context, and interceptors. |
| `demo` | Combined-config local verification app and consumer integration examples. |

## Configuration Lifecycle

The SDK deliberately separates parsing from runtime construction:

| Phase | MongoDB | Redis | gRPC |
| --- | --- | --- | --- |
| Parse | `mongo.LoadConfig` selects `framework.mongo`. | `redis.LoadConfig` selects `framework.redis`. | `atlasgrpc.LoadConfig` selects `framework.grpc`. |
| Enrich | Register entity/datastore/collection mappings in Go. | Optionally override fields in Go. | Optionally add/override named channels or code interceptors. |
| Construct | `mongo.Connect` creates cluster clients/datastores and auto-indexes registered entities. | `redis.NewClient` creates a lazy go-redis client. | `grpc.NewServer` creates the server; `NewChannelFactory` creates lazy named client connections. |

The three loaders can read the same project file but remain separate because each integration is an optional Go module. `${ENV:default}` expansion is supported by every loader. Manual construction remains a public compatibility contract and must not depend on YAML.

## Auth And Web Flow

The common auth boundary follows the Java SDK:

1. `web.ContextMiddleware` creates a `web.Context` from request headers, locale, remote IP, and query args.
2. Token preview fields are extracted with the same cookie-style keys as Java: `_u_`, `_d_`, `_s_`, `_exp_`, `_perms_`.
3. Configured `web.ContextCustomizer` functions can add business attributes, such as trace IDs, tenant hints, or routing metadata.
4. `sdk.RequireAuth(...)` or `Auth.RequiredByDefault` resolves the token from configured request headers and calls `auth.Service.Authenticate`.
5. Custom `web.Context.Attributes` are copied into `auth.Request.Attributes`.
6. Domain and permission checks are applied after identity resolution.
7. Auth failures return HTTP 200 with unified `{code,msg,data}` payloads, matching the Java SDK behavior.
8. The resolved `web.Context` is stored both in Gin context and `request.Context()` so downstream gRPC clients can propagate identity metadata.

## Response Model

`web.ResponseMiddleware` wraps normal JSON and text responses into `common.Response[T]` unless the path is excluded, the payload is already a response envelope, the status is non-2xx, or the content is streaming/binary.

Handlers can also call `web.OK` and `web.Fail` explicitly.

## Optional Data Integrations

MongoDB and Redis are not imported by the core module:

- `integrations/mongo` exposes `Connect`, `LoadConfig`, `Registry`, and `CRUD[T]`.
- MongoDB preserves named multi-cluster routing and supports nested `cluster -> datastores` configuration for multiple databases on one cluster.
- Registered entities map a Go type to a datastore and collection. Simple `mongo` field tags and entity-level `MongoIndexes` declarations are converted to ordered single-field or compound indexes when effective `auto-index` is enabled.
- Entity-level `CompoundIndex` is the Go counterpart of Morphia `@Indexes`; it preserves key order/direction and supports unique, sparse, hidden, and TTL options.
- `integrations/redis` exposes `LoadConfig`, `NewClient`, and `KeyBuilder`; `framework.redis` maps Java-style timeout and pool settings to go-redis.

Consumers add these modules only when needed from `github.com/ISeekFree/AtlasGo/integrations/...`. Local `replace` directives are reserved for development inside this repository.

## gRPC Flow

Server side:

- `UnaryServerAuthInterceptor` and `StreamServerAuthInterceptor` read `accessToken`, `authorization`, and optional `grpcToken` metadata.
- Requests authenticate through the same `auth.Service` contract.
- Resolved identity is stored in gRPC `context.Context` and read with `UserIDFromContext`, `IdentityFromContext`, or `AuthFromContext`.

Client side:

- `LoadConfig` reads the gRPC server listener and multiple named client service groups from `framework.grpc`.
- `NewChannelFactory` creates named channels from either loaded configuration or explicit programmatic options.
- Each channel name represents a service group with an independent target, plaintext mode, dial timeout, and inbound message limit. Connections are created only when requested.
- Client interceptors read the current `web.Context` from `context.Context`, resolve the token from the original HTTP request headers, and forward it as gRPC metadata.

This keeps Web and gRPC on one auth contract.

## Demo Validation

The `demo` module validates:

- public Gin routes and response wrapping;
- route-level auth with domain and permission checks;
- gRPC server/client auth propagation from WebContext;
- combined MongoDB, Redis, and multi-service gRPC YAML loading;
- generated protobuf `Echo` and `CurrentUser` RPC usage through the configured `local` channel;
- WebContext customization through `demoTraceId`;
- optional MongoDB and Redis setup paths and opt-in external basic operation tests;
- GitHub-based consumer installation instructions; local `replace` directives are used only for repository development.
