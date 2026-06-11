package middleware

import (
	"net/http"
	"slices"
)

// Cors returns a middleware that sets CORS headers for the given allowed
// origins. A single "*" entry allows any origin; otherwise the request's
// Origin header must match one of the entries exactly.
func Cors(allowedOrigins []string) Middleware {
	allowAll := slices.Contains(allowedOrigins, "*")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			switch {
			case allowAll:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case origin != "" && slices.Contains(allowedOrigins, origin):
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// The response depends on the Origin header, so caches must
				// key on it.
				w.Header().Add("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
