package middleware

import (
	"microservice/pkg/apperror"
	"microservice/pkg/response"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// RateLimit returns a global token-bucket rate-limit middleware.
// rate = requests/window; burst = requests.
func RateLimit(requests int, window time.Duration) Middleware {
	rps := rate.Limit(float64(requests) / window.Seconds())
	limiter := rate.NewLimiter(rps, requests)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
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
