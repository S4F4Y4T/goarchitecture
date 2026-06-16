package middleware

import (
	"context"
	"net/http"
	"strconv"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/response"
)

type contextKey string

const userIDKey contextKey = "user_id"

// Auth reads X-User-ID injected by Kong after JWT verification.
func Auth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userIDStr := r.Header.Get("X-User-ID")
			if userIDStr == "" {
				response.Error(w, r, apperror.Unauthorized("missing or invalid authorization header"))
				return
			}
			userID, err := strconv.Atoi(userIDStr)
			if err != nil || userID <= 0 {
				response.Error(w, r, apperror.Unauthorized("invalid user ID"))
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
		})
	}
}

func GetUserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}
