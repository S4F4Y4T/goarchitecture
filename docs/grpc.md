# gRPC (East-West Communication)

## Why

`auth` and `user` used to share one Postgres database directly — `auth` had its own GORM repository querying the same `users` table the `user` service owned. That only worked because they were never truly independent: a schema change in `user` could silently break `auth`. Once they became separate deployable services, `auth` needed a real way to ask `user` "does this email exist?" / "give me this user" / "create this user" without touching its database.

gRPC was chosen over HTTP+JSON for this one internal hop:
- The contract is typed and generated (`pkg/proto/user/user.proto`) — no hand-maintained request/response structs drifting out of sync between services.
- Binary framing (protobuf over HTTP/2) is cheaper to encode/decode than JSON for high-frequency internal calls (every login/register goes through this).

External, client-facing APIs stay on REST/JSON through Kong. gRPC is used **only** for this one internal, service-to-service call — see [microservice.md](microservice.md) for the broader communication strategy.

## Contract

```
pkg/proto/user/
├── user.proto          hand-written source of truth
├── user.pb.go          generated message types
└── user_grpc.pb.go     generated client/server stubs
```

```protobuf
service UserService {
  rpc ExistsByEmail(ExistsByEmailRequest) returns (ExistsByEmailResponse);
  rpc GetByEmail(GetByEmailRequest) returns (UserResponse);
  rpc Create(CreateRequest) returns (UserResponse);
}
```

These three RPCs are deliberately narrow — they're exactly the three methods `auth.UserLookup` needs (see [user-service-architecture.md](user-service-architecture.md) for how that interface was scoped before the services even split). `pkg/proto` lives in the shared `pkg` Go module so both services import the same generated types without a circular dependency between `services/auth` and `services/user`.

`UserResponse.created_at` / `updated_at` are RFC3339 strings, not `google.protobuf.Timestamp` — avoids pulling in the well-known-types dependency for two fields nobody parses into anything but a Go `time.Time`.

## Server Side — `user` service

`services/user/internal/user/grpc_server.go` implements `pb.UserServiceServer` and calls straight into the same `Repository` the HTTP handlers use — there's no separate gRPC-only business logic.

`services/user/cmd/api/main.go` runs the gRPC server **alongside** the HTTP server, on its own port (`GRPC_PORT`, separate from `PORT`):

```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(recoveryInterceptor))
pb.RegisterUserServiceServer(grpcServer, a.UserGRPCServer)
lis, _ := net.Listen("tcp", ":"+strconv.Itoa(cfg.GRPCPort))
go grpcServer.Serve(lis)
```

`recoveryInterceptor` is the gRPC equivalent of `pkgmiddleware.PanicRecovery` on the HTTP side — a panic in one RPC handler becomes an `Internal` error instead of crashing the whole process (which would also take down the HTTP server, since they're one binary). `grpcServer.GracefulStop()` is called during shutdown, same spirit as `http.Server.Shutdown`.

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
userConn, _ := grpc.NewClient(cfg.UserGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
userClient := pb.NewUserServiceClient(userConn)
```

`grpc.NewClient` is lazy — it doesn't dial until the first RPC.

### Layout convention: `internal/clients/<service>/`

Outbound gRPC clients live under `internal/clients/<service>/`, one package per remote dependency, named after the service it talks to (`clients/user/`, and `clients/catalog/` whenever auth or any other service needs to call catalog). The package itself keeps a plain name (`user`, not `userclient`) — the `clients/` parent already supplies the "this is a remote dependency, not a domain I own" context, so the leaf name doesn't need to repeat it.

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

This covers the "server + client alongside HTTP" half of the [next.md](next.md) Phase 4 checklist. Deliberately not yet done:

- **No per-call deadline.** RPCs use whatever context the HTTP handler passed in, which has no explicit timeout. A hung `user` service currently hangs the calling `auth` request instead of failing fast.
- **No client-side interceptors.** The server has panic recovery; the client has no logging, no metrics, and doesn't propagate the HTTP request ID into gRPC metadata — correlating a failed `auth` request with the matching `user` service log line currently requires matching timestamps by hand.
- **Readiness doesn't reflect the dependency.** `auth`'s `/readyz` can return `200` even when `user_app` is completely unreachable.
- **No load-balancing policy.** Fine for a single `user_app` container; `pick_first` (the default) won't spread load across replicas if `user` is ever scaled horizontally.
- **No `grpc_health_v1` health protocol or reflection** — not needed yet since there's no load balancer probing gRPC directly and no use of `grpcurl`/`grpcui` in this workflow.

## Alternatives Considered

- **HTTP+JSON for this call too** — simpler, one less toolchain (no `protoc`). Rejected for this specific hop because it's the highest-frequency internal call (every login/register) and the contract benefits from generated, typed stubs instead of hand-kept structs on both sides.
- **Shared database (status quo before this change)** — no network call at all, lowest latency. Rejected on purpose: it was the reason `auth` and `user` couldn't be deployed or scaled independently.
- **Async (publish "user created" event, auth reads from its own projection)** — removes the synchronous dependency entirely. Deferred: `Login`/`Register` need a synchronous answer ("does this email exist *right now*"), so an eventually-consistent read model doesn't fit this use case as cleanly as it would for, say, a search index.
