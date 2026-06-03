package middleweare

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// RequestIDHeader is the canonical header used to carry the request ID, both
// inbound (for cross-service tracing) and on the response.
const RequestIDHeader = "X-Request-ID"

type contextKey string

// RequestIDKey is the context key under which the per-request ID is stored.
const RequestIDKey contextKey = "request_id"

// RequestID ensures every request carries a correlation ID. It reuses an
// incoming X-Request-ID header when present (important for tracing across
// services) or generates a new UUID, sets it on the response header, and
// injects it into the request context so logs and downstream calls can use it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1. reuse incoming ID if exists (important for tracing across services)
		reqID := r.Header.Get(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.NewString()
		}

		// 2. set response header
		w.Header().Set(RequestIDHeader, reqID)

		// 3. inject into context
		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID stored in ctx, or "" if none is set.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey).(string)
	return id
}
