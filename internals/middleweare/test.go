package middleweare

import (
	"log"
	"net/http"
)

// Test is a throwaway middleware for verifying per-route wiring. It logs the
// request and sets a marker header so you can confirm it only runs on the
// routes it's explicitly attached to.
func Test(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[test middleware] hit %s %s", r.Method, r.URL.Path)
		w.Header().Set("X-Test-Middleware", "ok")
		next.ServeHTTP(w, r)
	})
}
