package middleware

import (
	"context"
	"log/slog"
	"microservice/pkg/apperror"
	"microservice/pkg/response"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// incrScript increments the counter and sets the TTL only on the first request
// in each window, keeping it atomic.
var incrScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// RateLimit returns a fixed-window per-IP rate-limit middleware backed by Redis.
// When rdb is nil (Redis not configured) the middleware is a no-op.
// On Redis errors the middleware fails open so the service stays available.
func RateLimit(rdb *redis.Client, requests int, window time.Duration) Middleware {
	if rdb == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	windowSec := strconv.FormatInt(int64(window.Seconds()), 10)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "rl:" + clientIP(r)

			count, err := incrScript.Run(
				context.Background(),
				rdb,
				[]string{key},
				windowSec,
			).Int64()

			if err != nil {
				slog.Warn("rate limiter redis error, failing open", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if count > int64(requests) {
				w.Header().Set("Retry-After", windowSec)
				response.JSONResponse(w, http.StatusTooManyRequests, response.ApiResponse{
					Success: false,
					Error: &response.ErrorBody{
						Code:    apperror.CodeTooManyRequests,
						Message: "too many requests",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the real client IP, respecting common proxy headers.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
