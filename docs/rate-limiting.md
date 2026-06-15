# Rate Limiting

## Implementation: Redis Fixed-Window, Per-IP

`pkg/middleware/ratelimit.go` implements a fixed-window counter per client IP, backed by Redis.

```go
middleware.RateLimit(rdb, "global", 100, time.Minute)
// allows 100 requests per minute per IP
```

**Algorithm**: For each request, a Lua script atomically:
1. Increments a counter key `rl:<namespace>:<client_ip>`
2. On the first increment, sets the TTL to the window duration

If the counter exceeds the limit, the middleware returns `429 Too Many Requests` with a `Retry-After` header (seconds until the window resets).

**Lua script (atomic INCR + EXPIRE)**:
```lua
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
```

Using Lua ensures the INCR and EXPIRE are atomic — no race condition between two concurrent requests on the first hit of a window.

## IP Detection

Client IP is extracted from (in order of preference):
1. `X-Real-IP` header (set by nginx when proxying)
2. First IP in `X-Forwarded-For` header (set by load balancers)
3. `r.RemoteAddr` (direct connection)

## Fail-Open Behavior

If Redis is unavailable (connection refused, timeout), the middleware:
- Logs a warning
- **Allows the request through** (fails open)

This is a deliberate availability-over-safety tradeoff. Rate limiting is a best-effort defense; losing it briefly during a Redis outage is preferable to taking down the API entirely.

## Disabled When Redis Is Not Configured

If `REDIS_ADDR` is not set in the environment, `SetupRedis()` returns `nil`. `RateLimit(nil, ...)` returns a no-op middleware — rate limiting is simply absent. Services can run without Redis in development or environments that don't need rate limiting.

## Configuration

```env
REDIS_ADDR=localhost:6380
REDIS_PASSWORD=          # optional
REDIS_DB=0               # optional
RATE_LIMIT_REQUESTS=100  # default: 100
RATE_LIMIT_WINDOW=1m     # default: 1 minute
```

## Why Fixed-Window?

Fixed-window is the simplest correct algorithm for rate limiting. It has one known weakness: a client can send `2N` requests in a short window if they burst at the end of one window and the start of the next.

Sliding-window algorithms (sliding log, sliding counter) eliminate the burst problem but require more Redis storage and computation. Fixed-window is sufficient for abuse prevention — it's not designed to be mathematically precise.

## Why Redis (not in-memory)?

In-memory counters are per-process. In a multi-instance deployment (multiple pods behind a load balancer), each pod has its own counter. A client can make `N * num_pods` requests before being rate-limited. Redis provides a shared counter across all instances.

**Tradeoff**: Redis is an added infrastructure dependency. If you have one instance and Redis is unavailable, you have no rate limiting. The fail-open behavior handles this gracefully.

## Alternatives Considered

- **In-memory (`sync.Map` + TTL)** — no Redis required, but ineffective in multi-instance deploys. Suitable for single-process deployments only.
- **Sliding window log** — tracks exact timestamps of each request. More accurate, more memory per client. Not needed for per-minute abuse prevention.
- **Token bucket** — allows short bursts above the average rate. More user-friendly (burst headroom). More complex to implement atomically in Redis. Deferred.
- **golang.org/x/time/rate** — in-memory token bucket. Same problem as in-memory counters (per-process only).
- **API gateway rate limiting (Kong, nginx)** — push rate limiting to the edge. Better when all services share the same limits. **This is now the active approach** — Kong handles rate limiting before requests reach the services. The service-level `RateLimit` middleware is commented out. See [api-gateway.md](api-gateway.md).
