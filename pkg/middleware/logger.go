package middleware

import (
	"net/http"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/logger"
)

// statusRecorder wraps http.ResponseWriter to capture the status code written
// by the handler, so the access log can report it. It defaults to 200, which
// is what net/http assumes when a handler writes a body without calling
// WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// Logger attaches a request-scoped slog logger (carrying the request ID) to the
// context so handlers and downstream middleware log with correlation, and emits
// a structured access log once the request completes, including the response
// status and latency.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		reqLogger := logger.L().With("request_id", GetRequestID(r.Context()))
		ctx := logger.WithContext(r.Context(), reqLogger)
		r = r.WithContext(ctx)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		reqLogger.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
