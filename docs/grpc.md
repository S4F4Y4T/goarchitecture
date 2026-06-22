# gRPC (East-West Communication)

## Why

`auth` and `user` used to share one Postgres database directly — `auth` had its own GORM repository querying the same `users` table the `user` service owned. That only worked because they were never truly independent: a schema change in `user` could silently break `auth`. Once they became separate deployable services, `auth` needed a real way to ask `user` "does this email exist?" / "give me this user" / "create this user" without touching its database.

gRPC was chosen over HTTP+JSON for this one internal hop:
- The contract is typed and generated (`pkg/proto/user/user.proto`) — no hand-maintained request/response structs drifting out of sync between services.
- Binary framing (protobuf over HTTP/2) is cheaper to encode/decode than JSON for high-frequency internal calls (every login/register goes through this).

External, client-facing APIs stay on REST/JSON through Kong. gRPC is used **only** for this one internal, service-to-service call — see [microservice.md](microservice.md) for the broader communication strategy.

## Contract

```
pkg/proto/
├── validate/
│   └── validate.proto       vendored protoc-gen-validate well-known proto
└── user/
    ├── user.proto            hand-written source of truth
    ├── user.pb.go            generated message types
    ├── user.pb.validate.go   generated PGV Validate() methods
    └── user_grpc.pb.go       generated client/server stubs
```

```protobuf
service UserService {
  rpc ExistsByEmail(ExistsByEmailRequest) returns (ExistsByEmailResponse);
  rpc GetByEmail(GetByEmailRequest) returns (UserResponse);
  rpc Create(CreateRequest) returns (UserResponse);
}

message CreateRequest {
  string name     = 1 [(validate.rules).string.min_len = 1];
  string email    = 2 [(validate.rules).string.email = true];
  string password = 3 [(validate.rules).string.min_len = 8];
}
```

These three RPCs are deliberately narrow — they're exactly the three methods `auth.UserLookup` needs (see [user-service-architecture.md](user-service-architecture.md) for how that interface was scoped before the services even split). `pkg/proto` lives in the shared `pkg` Go module so both services import the same generated types without a circular dependency between `services/auth` and `services/user`.

`UserResponse.created_at` / `updated_at` are RFC3339 strings, not `google.protobuf.Timestamp` — avoids pulling in the well-known-types dependency for two fields nobody parses into anything but a Go `time.Time`.

### Regenerating: `make proto`

```bash
make proto
```

Runs `protoc` (with `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-validate`) over every `*.proto` under `pkg/proto`, using each file's own directory as both the import root and output directory (plus `pkg/proto` as a secondary import root, so `import "validate/validate.proto";` resolves). Adding a second service's proto (e.g. `pkg/proto/video/video.proto`) needs no Makefile changes — the target globs for `.proto` files automatically.

### Field validation (protoc-gen-validate)

`(validate.rules)` annotations in the `.proto` generate a `Validate() error` method on every message (`user.pb.validate.go`). `pkg/grpcmiddleware.Validation` enforces this generically for any request type that implements it — one interceptor, no per-RPC boilerplate, the gRPC-side equivalent of `pkg/validation` on the HTTP side:

```go
if v, ok := req.(interface{ Validate() error }); ok {
    if err := v.Validate(); err != nil {
        return nil, status.Error(codes.InvalidArgument, err.Error())
    }
}
```

A bad request (invalid email, password under 8 characters, empty name) is rejected with `codes.InvalidArgument` and a specific message (e.g. `invalid CreateRequest.Password: value length must be at least 8 runes`) before it ever reaches `GRPCServer`'s methods — `grpc_server.go` itself doesn't need to know validation happened.

## Server Side — `user` service

`services/user/internal/user/grpc_server.go` implements `pb.UserServiceServer` and calls straight into the same `Repository` the HTTP handlers use — there's no separate gRPC-only business logic.

`services/user/cmd/api/main.go` runs the gRPC server **alongside** the HTTP server, on its own port (`GRPC_PORT`, separate from `PORT`):

```go
grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        grpcmiddleware.RequestID,
        grpcmiddleware.Logger,
        grpcmiddleware.Recovery,
        grpcmiddleware.Validation,
    ),
    grpc.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second, Timeout: 10 * time.Second}),
)
pb.RegisterUserServiceServer(grpcServer, a.UserGRPCServer)
lis, _ := net.Listen("tcp", ":"+strconv.Itoa(cfg.GRPCPort))
go grpcServer.Serve(lis)
```

Interceptor order matters — `RequestID` must run outermost, before `Logger`/`Recovery`, since it's what populates the request-scoped logger they both read via `logger.FromContext(ctx)`. `Validation` runs innermost (closest to the handler) so a rejected request is still logged with its real outcome (`InvalidArgument`) and duration like any other call, not bypassed.

`grpcmiddleware.Recovery` is the gRPC equivalent of `pkgmiddleware.PanicRecovery` on the HTTP side — a panic in one RPC handler becomes an `Internal` error instead of crashing the whole process (which would also take down the HTTP server, since they're one binary). `grpcServer.GracefulStop()` (raced against the shutdown deadline via `context.AfterFunc`, falling back to `Stop()`) is called during shutdown, same spirit as `http.Server.Shutdown`.

### Request ID propagation

`pkg/grpcmiddleware.RequestID` (server) and `PropagateRequestID` (client) carry the same `X-Request-ID` that the HTTP middleware chain establishes at the edge across the gRPC hop, using the `x-request-id` gRPC metadata key:

- **Client** (`auth`, dial option): reads `middleware.GetRequestID(ctx)` — already in context because the same `ctx` flows unbroken from the HTTP handler down through `auth.Service` to the gRPC call — and attaches it to outgoing metadata.
- **Server** (`user`, interceptor): reads `x-request-id` from incoming metadata (or mints a UUID if absent, e.g. when called by something other than `auth`), then populates *both* `middleware.WithRequestID` and a request-scoped `logger.WithContext` logger — so `Logger` and `Recovery` start including `request_id` in their log lines for free, no changes needed to either.

The payoff: a single request ID now threads through both services' logs for one logical request — `grep <request_id>` across `auth_app` and `user_app` reconstructs the whole story, including which of `ExistsByEmail`/`Create` ran and how long each took.

### Health check (`grpc_health_v1`)

`user`'s gRPC server also registers the standard `google.golang.org/grpc/health` service, so orchestrators and tools (`grpcurl`, k8s gRPC probes) can query readiness via the canonical `grpc.health.v1.Health/Check` RPC instead of an ad-hoc one:

```go
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
healthServer.SetServingStatus(pb.UserService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
```

This sets a static `SERVING` status once at startup — grpcmiddlewareit is **not** wired to live DB health the way the HTTP `/readyz` endpoint is. A request for an unregistered service name correctly returns `codes.NotFound` (the standard library's own behavior, not custom code here).

### Error mapping (domain → wire)

`pkg/apperror.AppError` has a `Code` (`NOT_FOUND`, `CONFLICT`, `INVALID_INPUT`, ...) that maps to HTTP status codes for the REST API. Crossing a gRPC boundary, the same `Code` maps to a `google.golang.org/grpc/codes.Code` instead, so the semantic meaning survives the network hop instead of flattening into a generic error:

| `apperror.Code` | gRPC `codes.Code` |
|---|---|
| `NOT_FOUND` | `NotFound` |
| `CONFLICT` | `AlreadyExists` |
| `INVALID_INPUT` | `InvalidArgument` |
| `UNAUTHORIZED` | `Unauthenticated` |
| `FORBIDDEN` | `PermissionDenied` |
| (anything else) | `Internal` |

## Client Side — `auth` service

`services/auth/internal/clients/user/client.go` wraps the generated `pb.UserServiceClient` stub in a `Client` type that implements `auth.UserLookup`:

```go
type UserLookup interface {
    ExistsByEmail(ctx context.Context, email string) (bool, error)
    GetByEmail(ctx context.Context, email string) (*user.User, error)
    Create(ctx context.Context, u *user.User) (*user.User, error)
}
```

`auth.Service` depends only on this interface — it has no idea gRPC is involved. Swapping the old GORM-backed repository for the gRPC client was a one-line change at the composition root (`app.go`); `auth.Service`'s code didn't change at all.

The connection is established once at startup in `services/auth/cmd/api/main.go` and reused for the process lifetime (gRPC connections are HTTP/2 and meant to be long-lived, not dialed per request):

```go
userConn, _ := grpc.NewClient(
    cfg.UserGRPCAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithChainUnaryInterceptor(grpcmiddleware.PropagateRequestID),
)
userClient := pb.NewUserServiceClient(userConn)
```

`grpc.NewClient` is lazy — it doesn't dial until the first RPC.

### Layout convention: `internal/clients/<service>/`

Outbound gRPC clients live under `internal/clients/<service>/`, one package per remote dependency, named after the service it talks to (`clients/user/` today; e.g. `clients/video/` for any future service that needs to call a `video` service). The package itself keeps a plain name (`user`, not `userclient`) — the `clients/` parent already supplies the "this is a remote dependency, not a domain I own" context, so the leaf name doesn't need to repeat it.

This is distinct from `services/user/internal/user/` — that's the real domain (model, repository, service, handlers); `services/auth/internal/clients/user/` is a thin adapter (one `Client` struct, one DTO) that happens to share a package name because it represents the same concept from the consuming side.

### Error mapping (wire → domain)

`fromGRPCError` in `client.go` reverses the server-side table above: a `codes.AlreadyExists` response becomes `apperror.Conflict(...)` again, so `auth.Service.Register` returns the same `409` it always did, regardless of whether the duplicate-email check happened in a local DB call or a remote one.

## Transport Security

Connections use `insecure.NewCredentials()` — plaintext, no TLS. This is a deliberate tradeoff, not an oversight: `auth_app` and `user_app` only talk to each other over the internal Docker network (`expose`, not `ports` — see [docker.md](docker.md)), which isn't reachable from outside the network. Trusting that network boundary is acceptable for the current single-host Docker Compose deployment.

This does **not** carry over to Kubernetes / multi-node, where pod-to-pod traffic can traverse shared infrastructure. At that point this needs mTLS (cert-manager or SPIFFE/SPIRE, ideally handled transparently by a sidecar rather than application code) — tracked in [next.md](next.md) Phase 4.

## Configuration

| Env var | Service | Meaning |
|---|---|---|
| `GRPC_PORT` | user | Port the gRPC server listens on (separate from `PORT`, the HTTP port) |
| `USER_GRPC_ADDR` | auth | `host:port` of the user service's gRPC server (`user_app:6970` in Docker Compose) |

`auth` no longer has any `DB_*` env vars or a Postgres connection at all — `USER_GRPC_ADDR` is its only path to user data. Its `/readyz` checks Redis only; it does **not** currently verify the gRPC connection to `user` is healthy (see Known Gaps below).

## Known Gaps

This covers the [next.md](next.md) Phase 4 checklist. Deliberately not yet done:

- **No per-call deadline.** RPCs use whatever context the HTTP handler passed in, which has no explicit timeout. A hung `user` service currently hangs the calling `auth` request instead of failing fast.
- **Readiness doesn't reflect the dependency.** `auth`'s `/readyz` can return `200` even when `user_app` is completely unreachable. Relatedly, `user`'s `grpc_health_v1` status is static (set once at startup), not wired to a live DB ping the way HTTP's `/readyz` is.
- **No load-balancing policy.** Fine for a single `user_app` container; `pick_first` (the default) won't spread load across replicas if `user` is ever scaled horizontally.
- **No gRPC reflection.** `grpcurl`/`grpcui` need `-proto` passed explicitly instead of querying the server for its schema — confirmed by hand while testing the other fixes in this list.

Request ID propagation, structured logging, panic recovery, and field validation (PGV) are now implemented — see the sections above.

## Alternatives Considered

- **HTTP+JSON for this call too** — simpler, one less toolchain (no `protoc`). Rejected for this specific hop because it's the highest-frequency internal call (every login/register) and the contract benefits from generated, typed stubs instead of hand-kept structs on both sides.
- **Shared database (status quo before this change)** — no network call at all, lowest latency. Rejected on purpose: it was the reason `auth` and `user` couldn't be deployed or scaled independently.
- **Async (publish "user created" event, auth reads from its own projection)** — removes the synchronous dependency entirely. Deferred: `Login`/`Register` need a synchronous answer ("does this email exist *right now*"), so an eventually-consistent read model doesn't fit this use case as cleanly as it would for, say, a search index.
