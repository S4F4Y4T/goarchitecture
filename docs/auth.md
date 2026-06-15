# Authentication

## Overview

Authentication is split into two token types issued on login and rotated on every refresh:

| Token | Type | Lifetime | Delivery |
|---|---|---|---|
| Access token | HS256 JWT | 15 min (configurable) | JSON response body |
| Refresh token | UUID v4 (opaque) | 7 days (configurable) | `httpOnly` cookie; stored in Redis |

The access token is short-lived and stateless — the server validates its signature without a database call. The refresh token is long-lived and stateful — Redis is the source of truth for whether it is still valid. Storing the refresh token in an `httpOnly` cookie means JavaScript cannot read it, eliminating the XSS theft vector.

## Flow

```
POST /v1/auth/register          → create account, return user (no tokens)
POST /v1/auth/login             → verify credentials
                                → body: { access_token, expires_in }
                                → cookie: Set-Cookie: refresh_token=<uuid>; HttpOnly; Path=/v1/auth
  ↓
Client stores access token (memory / localStorage)
Browser stores refresh token cookie automatically
  ↓
GET  /v1/users/                 → Authorization: Bearer <access_token>
  ↓ (access token expires after 15 min)
POST /v1/auth/refresh           → cookie sent automatically by browser
                                → body: { access_token, expires_in }
                                → cookie: new refresh_token replaces old
  ↓
POST /v1/auth/logout            → cookie sent automatically
                                → clears cookie (Max-Age=-1), deletes from Redis
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
decode LoginDTO → validate → service.Login → issueTokenPair → 200 + access token body + refresh cookie
```

`service.Login` returns the same `"invalid email or password"` error regardless of whether the email doesn't exist or the password is wrong. This prevents email enumeration.

### Refresh (token rotation)

```
read cookie → tokenStore.UserID → tokenStore.Delete → issueTokenPair → 200 + access token body + new refresh cookie
```

The old refresh token is deleted from Redis **before** the new pair is issued. This means:
- A stolen token can only be used once — the legitimate client's next refresh will fail, revealing the theft.
- There is no window where both the old and new tokens are simultaneously valid.

`POST /v1/auth/refresh` is a **public route** — no JWT required. The browser sends the cookie automatically; Redis validates the opaque value.

### Logout

```
read cookie → tokenStore.Delete (best-effort) → clear cookie (Max-Age=-1) → 204
```

The delete error is discarded so logout is idempotent. If no cookie is present, the cookie-clearing header is still sent and `204` is returned. The access token is not invalidated — it is short-lived enough that no server-side revocation is needed.

### `issueTokenPair`

Shared by Login and Refresh. Sets the cookie directly on the response writer:

```go
func (h *AuthHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, userID int) (map[string]any, error) {
    accessToken, _ := token.Generate(userID, h.jwtSecret, h.accessExpiry)
    refreshToken   := uuid.NewString()
    h.tokenStore.Save(r.Context(), refreshToken, userID, h.refreshExpiry)
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    refreshToken,
        HttpOnly: true,
        Secure:   h.cookieSecure,
        SameSite: http.SameSiteStrictMode,
        Path:     "/v1/auth",
        MaxAge:   int(h.refreshExpiry.Seconds()),
    })
    return map[string]any{
        "access_token": accessToken,
        "expires_in":   int(h.accessExpiry.Seconds()),
    }, nil
}
```

The cookie `Path` is scoped to `/v1/auth` so the browser never sends it alongside regular API calls to `/v1/users/` etc.

## Configuration

```
JWT_SECRET=change-me-in-production   # HMAC signing key
JWT_ACCESS_EXPIRY=15m                # access token lifetime
JWT_REFRESH_EXPIRY=168h              # refresh token lifetime (Redis TTL)
COOKIE_SECURE=false                  # set true in production (requires HTTPS)
```

`COOKIE_SECURE` defaults to `true`. Set it to `false` for local HTTP development — `Secure` cookies are not sent over plain HTTP.

## Security Properties

| Property | Mechanism |
|---|---|
| Access token integrity | HS256 signature; rejected if tampered or expired |
| Refresh token XSS resistance | `httpOnly` cookie — JavaScript cannot read the token |
| Refresh token CSRF resistance | `SameSite=Strict` — cookie not sent on cross-site requests |
| Refresh token secrecy | UUID is unguessable; only valid if present in Redis |
| Refresh token replay prevention | Rotation: old token deleted before new pair issued |
| Email enumeration prevention | Same error for wrong email and wrong password |
| Password storage | bcrypt with cost 10; raw password never logged or stored |
| Session revocation | Logout deletes from Redis and expires the cookie |

## Alternatives Considered

- **Stateless refresh tokens (signed JWT)** — no Redis dependency. Downside: impossible to revoke before expiry. A stolen refresh token would be valid for its full 7-day lifetime with no remedy. Stateful refresh tokens allow immediate revocation via logout or key rotation.
- **Refresh token in response body** — simpler for mobile and non-browser clients, but exposes the token to JavaScript (XSS risk). The httpOnly cookie approach is implemented; mobile clients that can't use cookies can use an alternative endpoint or pass credentials differently.
- **`SameSite=None` cookie** — required if the SPA is on a different origin (`https://app.example.com` → `https://api.example.com`). Allows cross-site requests but requires `Secure=true` and HTTPS. Change `SameSiteStrictMode` to `SameSiteNoneMode` in `issueTokenPair` and `Logout` for this case.
- **`RS256` (asymmetric) JWT** — allows other services to verify tokens without sharing the signing secret. Worth adding when a second service needs to validate the same tokens. `HS256` is simpler for a single-service setup.
- **Separate `AuthService`** — implemented. `AuthService` owns `Register` and `Login`; `UserService` owns CRUD. Both share the same `model.UserRepository` instance, wired in bootstrap.
- **Token reuse detection** — on refresh, if the presented token was already rotated (i.e., Redis miss), revoke all sessions for that user. Not implemented; requires storing a per-user session list in addition to per-token keys.
