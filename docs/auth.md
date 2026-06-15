# Authentication

## Overview

Authentication is split into two token types issued on login and rotated on every refresh:

| Token | Type | Lifetime | Storage |
|---|---|---|---|
| Access token | HS256 JWT | 15 min (configurable) | Client only |
| Refresh token | UUID v4 (opaque) | 7 days (configurable) | Redis (`refresh:<uuid>`) |

The access token is short-lived and stateless — the server validates its signature without a database call. The refresh token is long-lived and stateful — Redis is the source of truth for whether it is still valid.

## Flow

```
POST /v1/auth/register          → create account, return user (no tokens)
POST /v1/auth/login             → verify credentials, issue access + refresh token pair
  ↓
Client stores both tokens
  ↓
GET  /v1/users/                 → Authorization: Bearer <access_token>
  ↓ (access token expires after 15 min)
POST /v1/auth/refresh           → { "refresh_token": "<uuid>" }
                                → delete old token, issue new pair (rotation)
  ↓
POST /v1/auth/logout            → { "refresh_token": "<uuid>" }
                                → delete from Redis, access token naturally expires
```

## Token Package (`pkg/token`)

JWT logic and the refresh token store interface live in `pkg/token` so they can be reused across all services.

### `pkg/token/jwt.go`

```go
func Generate(userID int, secret string, expiry time.Duration) (string, error)
func ParseUserID(tokenStr, secret string) (int, error)
```

`Generate` creates a signed HS256 JWT with a `uid` claim and an `exp` claim.
`ParseUserID` validates the signature and algorithm, then returns the `uid` claim as `int`.

Using a typed `Claims` struct (not `jwt.MapClaims`) avoids the float64 coercion that happens when numeric claims are decoded into `map[string]any`.

### `pkg/token/store.go`

```go
type Store interface {
    Save(ctx context.Context, token string, userID int, expiry time.Duration) error
    UserID(ctx context.Context, token string) (int, error)
    Delete(ctx context.Context, token string) error
}
```

The interface lives in `pkg` so any future service can plug in its own Redis client without importing the user service.

## Redis Token Store (`services/user/internal/repository/token.go`)

The concrete implementation stores refresh tokens in Redis with a TTL:

```
SET refresh:<uuid>  <userID>  EX <refreshExpiry>
```

- `Save` — called when issuing a new refresh token.
- `UserID` — called on `POST /auth/refresh`; a Redis miss maps to `401 Unauthorized`.
- `Delete` — called on rotation (refresh) and logout.

Redis key expiry is the enforcement mechanism for refresh token lifetime. No separate cleanup job is needed.

## Auth Middleware (`services/user/internal/middleware/auth.go`)

Protects routes that require a logged-in user:

```go
func Auth(secret string) func(http.Handler) http.Handler
func GetUserID(ctx context.Context) (int, bool)
```

The middleware:
1. Reads the `Authorization` header and strips the `Bearer ` prefix.
2. Calls `token.ParseUserID` to validate the JWT signature and expiry.
3. Stores the `userID` in request context via `GetUserID`.
4. Returns `401` immediately on any failure — the handler is never called.

Applied at the route group level, not per-handler:

```go
// router/user.go
mux.Handle("/users/", auth(http.StripPrefix("/users", userMux)))
```

## Auth Handler (`services/user/internal/handler/auth.go`)

### Register

```
decode RegisterDTO → validate → service.Register → 201 + user
```

Password is hashed with bcrypt (cost 10) inside `service.Register` before being written to the database. The raw password never touches the repository layer.

### Login

```
decode LoginDTO → validate → service.Login → issueTokenPair → 200 + token pair
```

`service.Login` returns the same `"invalid email or password"` error regardless of whether the email doesn't exist or the password is wrong. This prevents email enumeration.

### Refresh (token rotation)

```
decode RefreshRequest → validate → tokenStore.UserID → tokenStore.Delete → issueTokenPair → 200 + new pair
```

The old refresh token is deleted from Redis **before** the new pair is issued. This means:
- A stolen token can only be used once — the legitimate client's next refresh will fail, revealing the theft.
- There is no window where both the old and new tokens are simultaneously valid.

`POST /v1/auth/refresh` is a **public route** — no JWT required. Redis validates the opaque token; an unknown UUID gets an immediate `401`.

### Logout

```
decode LogoutRequest → validate → tokenStore.Delete (best-effort) → 204
```

The delete error is discarded so logout is idempotent. A client that sends the same refresh token twice (e.g., due to a retry) gets `204` both times. The access token is not invalidated — it is short-lived enough that no server-side revocation is needed.

### `issueTokenPair`

Shared by Login and Refresh:

```go
func (h *AuthHandler) issueTokenPair(r *http.Request, userID int) (map[string]any, error) {
    accessToken, _  := token.Generate(userID, h.jwtSecret, h.accessExpiry)
    refreshToken    := uuid.NewString()
    h.tokenStore.Save(r.Context(), refreshToken, userID, h.refreshExpiry)
    return map[string]any{
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "expires_in":    int(h.accessExpiry.Seconds()),
    }, nil
}
```

## Configuration

```
JWT_SECRET=change-me-in-production   # HMAC signing key
JWT_ACCESS_EXPIRY=15m                # access token lifetime
JWT_REFRESH_EXPIRY=168h              # refresh token lifetime (Redis TTL)
```

Parsed in `config.JWTConfig` via `time.ParseDuration`. The defaults are 15 minutes and 7 days respectively.

## Security Properties

| Property | Mechanism |
|---|---|
| Access token integrity | HS256 signature; rejected if tampered or expired |
| Refresh token secrecy | UUID is unguessable; only valid if present in Redis |
| Refresh token replay prevention | Rotation: old token deleted before new pair issued |
| Email enumeration prevention | Same error for wrong email and wrong password |
| Password storage | bcrypt with cost 10; raw password never logged or stored |
| Session revocation | Logout deletes refresh token; access token expires naturally |

## Alternatives Considered

- **Stateless refresh tokens (signed JWT)** — no Redis dependency. Downside: impossible to revoke before expiry. A stolen refresh token would be valid for its full 7-day lifetime with no remedy. Stateful refresh tokens allow immediate revocation via logout or key rotation.
- **Refresh token stored in `httpOnly` cookie** — protects against XSS stealing the token from JavaScript. Complicates CORS and mobile client usage. Left as a future option for browser-first deployments.
- **Separate `AuthService`** — isolating Register/Login logic from UserService. Reasonable for large services. Kept together here because Register and Login share `model.UserRepository` and the service is small.
- **`RS256` (asymmetric) JWT** — allows other services to verify tokens without sharing the signing secret. Worth adding when a second service needs to validate the same tokens. `HS256` is simpler for a single-service setup.
- **Token reuse detection** — on refresh, if the presented token was already rotated (i.e., Redis miss), revoke all sessions for that user. Not implemented; requires storing a per-user session list in addition to per-token keys.
