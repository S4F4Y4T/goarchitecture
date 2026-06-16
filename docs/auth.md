# Authentication

## Overview

Authentication is split into two token types issued on login and rotated on every refresh:

| Token | Type | Lifetime | Delivery |
|---|---|---|---|
| Access token | RS256 JWT | 15 min (configurable) | JSON response body |
| Refresh token | UUID v4 (opaque) | 7 days (configurable) | `httpOnly` cookie; stored in Redis |

The access token is short-lived and stateless — Kong verifies its signature at the gateway before the request reaches any service. The refresh token is long-lived and stateful — Redis is the source of truth for whether it is still valid. Storing the refresh token in an `httpOnly` cookie means JavaScript cannot read it, eliminating the XSS theft vector.

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
                                  Kong verifies RS256 signature (rejects if invalid/expired)
                                  Kong strips client X-User-ID, injects X-User-ID from claims
                                  Service reads X-User-ID header → user ID in context
  ↓ (access token expires after 15 min)
POST /v1/auth/refresh           → cookie sent automatically by browser
                                → body: { access_token, expires_in }
                                → cookie: new refresh_token replaces old
  ↓
POST /v1/auth/logout            → cookie sent automatically
                                → clears cookie (Max-Age=-1), deletes from Redis
```

## RS256 Key Pair

The user service signs tokens with an RSA private key. Kong and the service itself verify tokens with the corresponding RSA public key. The private key never leaves the user service.

```
deploy/kong/jwt.key      ← RSA private key (gitignored; user service only)
deploy/kong/jwt.key.pub  ← RSA public key  (committed; used by Kong and the service)
```

Generate a new key pair:
```bash
openssl genrsa -out deploy/kong/jwt.key 2048
openssl rsa -in deploy/kong/jwt.key -pubout -out deploy/kong/jwt.key.pub
```

The public key is embedded in `deploy/kong/kong.yml` under the `consumers` block so Kong can verify signatures without a database call.

Kong verifies the JWT at the gateway for `/v1/users` and `/v1/products`, then injects `X-User-ID` as a trusted header. The service-level `Auth` middleware reads this header to get the user ID — no JWT parsing in the service.

## Token Package (`pkg/token`)

JWT logic and the refresh token store interface live in `pkg/token` so they can be reused across all services.

### `pkg/token/jwt.go`

```go
const Issuer = "go-microservice"  // must match Kong consumer credential key

func Generate(userID int, privateKey *rsa.PrivateKey, expiry time.Duration) (string, error)
func ParseUserID(tokenStr string, publicKey *rsa.PublicKey) (int, error)
```

`Generate` creates a signed RS256 JWT with `iss`, `uid`, and `exp` claims. The `iss` value must match the `key` field of the Kong consumer credential so Kong can locate the right public key.

`ParseUserID` validates the algorithm (rejects non-RSA), verifies the signature, checks `exp`, and returns the `uid` claim as `int`.

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

## Redis Token Store (`services/user/internal/auth/token_store.go`)

The concrete implementation stores refresh tokens in Redis with a TTL:

```
SET refresh:<uuid>  <userID>  EX <refreshExpiry>
```

- `Save` — called when issuing a new refresh token.
- `UserID` — called on `POST /auth/refresh`; a Redis miss maps to `401 Unauthorized`.
- `Delete` — called on rotation (refresh) and logout.

Redis key expiry is the enforcement mechanism for refresh token lifetime. No separate cleanup job is needed.

## Auth Middleware (`pkg/middleware/auth.go`)

Protects routes that require a logged-in user:

```go
func Auth() func(http.Handler) http.Handler
func GetUserID(ctx context.Context) (int, bool)
```

Kong verifies the JWT signature and injects `X-User-ID` before the request reaches the service. The middleware:
1. Reads the `X-User-ID` header (injected by Kong's `post-function` plugin).
2. Parses it to `int` and rejects non-positive values with `401`.
3. Stores the `userID` in request context via `GetUserID`.
4. Returns `401` immediately on any failure — the handler is never called.

No JWT parsing or RSA key material is needed in the middleware. Kong strips any client-supplied `X-User-ID` header before injecting its own, so the header is trustworthy on the internal Docker network.

Applied at the route group level, not per-handler:

```go
// router/user.go
mux.Handle("/users/", auth(http.StripPrefix("/users", userMux)))
```

## Auth Handler (`services/user/internal/auth/handler.go`)

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

Shared by Login and Refresh. Signs the access token with the RSA private key and sets the refresh cookie:

```go
func (h *AuthHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, userID int) (map[string]any, error) {
    accessToken, _ := token.Generate(userID, h.privateKey, h.accessExpiry)
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
JWT_PRIVATE_KEY_PATH=/app/deploy/kong/jwt.key   # RSA private key — signs tokens (user service only)
JWT_ACCESS_EXPIRY=15m                            # access token lifetime
JWT_REFRESH_EXPIRY=168h                          # refresh token lifetime (Redis TTL)
COOKIE_SECURE=true                               # set false for local HTTP dev (Secure cookies require HTTPS)
```

The RSA public key is embedded in `deploy/kong/kong.yml` for Kong to use. The service has no public key env var — it trusts Kong's `X-User-ID` header instead of re-verifying tokens itself.

## Security Properties

| Property | Mechanism |
|---|---|
| Access token integrity | RS256 signature; Kong rejects invalid/expired tokens at the gateway |
| User ID propagation | Kong injects `X-User-ID` after verification; client-supplied value stripped first |
| Private key isolation | Only the user service holds the private key; public key is only in `kong.yml` |
| Refresh token XSS resistance | `httpOnly` cookie — JavaScript cannot read the token |
| Refresh token CSRF resistance | `SameSite=Strict` — cookie not sent on cross-site requests |
| Refresh token secrecy | UUID is unguessable; only valid if present in Redis |
| Refresh token replay prevention | Rotation: old token deleted before new pair issued |
| Email enumeration prevention | Same error for wrong email and wrong password |
| Password storage | bcrypt with cost 10; raw password never logged or stored |
| Session revocation | Logout deletes from Redis and expires the cookie |

## Alternatives Considered

- **HS256 (symmetric) JWT** — simpler, single shared secret. Kong can verify with the secret, but the secret must be distributed to every service that verifies tokens — any service compromise exposes the signing key. RS256 limits that: only the user service has the private key; all verifiers hold only the public key.
- **Stateless refresh tokens (signed JWT)** — no Redis dependency. Downside: impossible to revoke before expiry. A stolen refresh token would be valid for its full 7-day lifetime with no remedy. Stateful refresh tokens allow immediate revocation via logout or key rotation.
- **Refresh token in response body** — simpler for mobile and non-browser clients, but exposes the token to JavaScript (XSS risk). The httpOnly cookie approach is implemented; mobile clients that can't use cookies can use an alternative endpoint or pass credentials differently.
- **`SameSite=None` cookie** — required if the SPA is on a different origin (`https://app.example.com` → `https://api.example.com`). Allows cross-site requests but requires `Secure=true` and HTTPS. Change `SameSiteStrictMode` to `SameSiteNoneMode` in `issueTokenPair` and `Logout` for this case.
- **Service-level JWT re-verification** — the service could also verify the RS256 signature with the public key for defense-in-depth. Not done: the network boundary (`expose` not `ports`) and Kong's `request-transformer` stripping forged `X-User-ID` headers are the chosen trust boundary. Adding the public key to the service would create a second copy to rotate on key change.
- **Token reuse detection** — on refresh, if the presented token was already rotated (i.e., Redis miss), revoke all sessions for that user. Not implemented; requires storing a per-user session list in addition to per-token keys.
