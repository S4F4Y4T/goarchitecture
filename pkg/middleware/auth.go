package middleware

import (
	"context"
	"net/http"
	"strings"

	"microservice/pkg/apperror"
	"microservice/pkg/response"
	"microservice/pkg/token"
)

// ClaimsKey is the context key under which the verified token claims are
// stored by Authenticate.
const ClaimsKey contextKey = "auth_claims"

// Claims returns the authenticated caller's claims, or nil when the request
// did not pass through Authenticate.
func Claims(ctx context.Context) *token.Claims {
	c, _ := ctx.Value(ClaimsKey).(*token.Claims)
	return c
}

// Authenticate rejects requests that don't carry a valid "Authorization:
// Bearer <token>" header and injects the verified claims into the request
// context for handlers and later middleware.
func Authenticate(tokens *token.Manager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			raw, ok := strings.CutPrefix(authz, "Bearer ")
			if !ok || raw == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
				response.Error(w, r, apperror.Unauthorized("missing or malformed Authorization header"))
				return
			}

			claims, err := tokens.Verify(raw)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
				response.Error(w, r, apperror.Unauthorized("invalid or expired token"))
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole allows the request through only when the authenticated caller
// holds the given role. Must run after Authenticate.
func RequireRole(role string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := Claims(r.Context())
			if claims == nil || claims.Role != role {
				response.Error(w, r, apperror.Forbidden("insufficient permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
