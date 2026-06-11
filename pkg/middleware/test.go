package middleware

import (
	"net/http"

	"microservice/pkg/logger"
)

// Test is a throwaway middleware for verifying per-route wiring. It logs the
// request and sets a marker header so you can confirm it only runs on the
// routes it's explicitly attached to.
func Test(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).Info("test middleware hit", "method", r.Method, "path", r.URL.Path)
		w.Header().Set("X-Test-Middleware", "ok")
		next.ServeHTTP(w, r)
	})
}
