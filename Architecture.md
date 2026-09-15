# AtlasGo GoMVC Architecture

## Baseline

- Runtime target: Go 1.23.
- Web framework: Gin.
- Module root: `github.com/ISeekFree/AtlasGo`.
- Optional integrations are separate Go modules under `integrations/` so consumers only add the dependencies they use.

## Modules

| Module | Role |
| --- | --- |
| `auth` | JWT crypto only: `JWTCodec` and `StripBearer`. Business claim names never appear here. |
| `common` | SDK error type, common error codes, unified `Response<T>`, and `Paged<T>` payloads. |
| `web` | Gin middleware for WebContext, extension customizers, auth, CORS, response wrapping, and panic/error conversion; also the WebSocket base context (`WebsocketContext`). |
| `integrations/mongo` | MongoDB YAML loading, multi-cluster/datastore registry, entity routing/indexes, and CRUD. |
| `integrations/redis` | Redis YAML loading, client/pool option mapping, and key prefix helper. |
| `integrations/grpc` | gRPC YAML loading, named channels, metadata keys, and context helpers (`WithContext`/`ContextFromContext`). Auth interceptors are business-owned. |
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

0. `web.CorsMiddleware` runs first (installed by `sdk.Install`): it decorates every response and answers a preflight `OPTIONS` with `204`, so auth and error paths never need their own CORS handling. It is configured through `Options.Cors` (`AllowedOriginPatterns`, `AllowedMethods`, `AllowedHeaders`, `ExposedHeaders`, `AllowCredentials`, `MaxAge`) and disabled with `web.Bool(false)`.
1. `web.ContextMiddleware` creates a `web.Context` from request headers, locale, remote IP, and query args.
2. The SDK never previews token claims; `web.Context` user fields only appear after auth (or through a `web.ContextCustomizer`).
3. Configured `web.ContextCustomizer` functions can add business attributes, such as trace IDs, tenant hints, or routing metadata.
4. `ContextMiddleware` builds a `*web.Context` (optionally via `Options.ContextProvider`) and runs every `Options.ContextLoaders` entry against a transport-neutral `web.Request`. `sdk.RequireAuth(...)` or `Auth.RequiredByDefault` then checks the loaded `Context`: `UID` must be present, the `Domain` must match, and `Permissions` go to `Options.PermissionChecker` (absent ⇒ deny).
5. Custom `web.Context.Attributes` are copied into `auth.Request.Attributes`.
6. Domain and permission checks are applied after identity resolution.
7. Auth failures return HTTP 200 with unified `{code,msg,data}` payloads, matching the Java SDK behavior.
8. The resolved `web.Context` is stored both in Gin context and `request.Context()` so downstream code, including business gRPC client interceptors, can resolve the caller.

## Response Model

`web.ResponseMiddleware` wraps normal JSON and text responses into `common.Response[T]` unless the path is excluded, the payload is already a response envelope, the status is non-2xx, or the content is streaming/binary.

Failures use the same envelope. `common.Error` is the unified framework error: `common.NewError("msg")` / `common.WrapError("msg", cause)` default to `common.SystemErrorCode` (-90), while `common.NewErrorCode(code, "msg")` / `common.WrapErrorCode(code, "msg", cause)` carry an explicit business code. `web.Recovery` turns a panic into HTTP 200 with the error's code and message; handler-reported errors (`c.Error(...)`) render through `ResponseMiddleware` when the handler wrote no body. `web.Options.ErrorResolvers` runs before the SDK default, so a business can map any failure onto its own `{code,msg,data}`.

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

- Auth policy is owned by the business layer in both directions: each service registers its own `grpc.UnaryServerInterceptor`/`grpc.StreamServerInterceptor` via `grpc.NewServer(grpc.ChainUnaryInterceptor(...))`, and each client supplies `UnaryInterceptors`/`StreamInterceptors` through `atlasgrpc.ClientOptions`.
- AtlasGo exposes only the shared building blocks: the `atlasgrpc.Metadata*` keys, the `atlasgrpc.MetadataRequest` adapter onto `web.Request`, and `WithContext`/`ContextFromContext` to store and read the resolved `*web.Context`.
- The business interceptor reads `accessToken`, `authorization`, and optional `grpcToken` metadata, parses them through the same `web.ContextLoader` used for HTTP, and attaches the `*web.Context` with `WithContext`.
- Resolved context is stored in gRPC `context.Context` and read with `ContextFromContext` (or the `UserIDFromContext` convenience).

Client side:

- `LoadConfig` reads the gRPC server listener and multiple named client service groups from `framework.grpc`.
- `NewChannelFactory` creates named channels from either loaded configuration or explicit programmatic options.
- Each channel name represents a service group with an independent target, plaintext mode, dial timeout, and inbound message limit. Connections are created only when requested.
- The SDK does not install an auth interceptor. The consuming project supplies its own unary/stream client interceptors, decides where the token comes from, and writes the metadata keys its services expect.

Web, gRPC and WebSocket therefore share only the `web.Context`, `web.ContextLoader`, `web.ResolveToken`, the `Metadata*` keys, and the context helpers, not a fixed token-transport policy. Each transport has a base context you can use directly or embed: HTTP `web.Context` (Gin context / `request.Context()`), gRPC `atlasgrpc.WithContext` + `atlasgrpc.ContextFromContext`, and WebSocket `web.WebsocketContext` + `web.AttachWebsocketContext`/`web.WebsocketContextFrom` on the connection state.

Resolver targets use `static` by default when no scheme is present. The grpc-go
built-ins `dns`, `unix`, and `passthrough` remain available for explicit targets,
and AtlasGo imports the official `xds` resolver by default. xDS still requires
an xDS runtime/bootstrap configuration when an `xds:///...` target is used.

## Demo Validation

The `demo` module validates:

- public Gin routes and response wrapping;
- route-level auth with domain and permission checks;
- business-owned gRPC auth in both directions, wired through the demo's own interceptors;
- combined MongoDB, Redis, and multi-service gRPC YAML loading;
- generated protobuf `Echo` and `CurrentUser` RPC usage through the configured `local` channel;
- WebContext customization through `demoTraceId`;
- optional MongoDB and Redis setup paths and opt-in external basic operation tests;
- GitHub-based consumer installation instructions; local `replace` directives are used only for repository development.
