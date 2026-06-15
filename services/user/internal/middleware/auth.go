package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/response"
	"github.com/s4f4y4t/go-microservice/pkg/token"
)

type contextKey string

const userIDKey contextKey = "user_id"

func Auth(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, r, apperror.Unauthorized("missing or invalid authorization header"))
				return
			}
			userID, err := token.ParseUserID(strings.TrimPrefix(authHeader, "Bearer "), publicKey)
			if err != nil {
				response.Error(w, r, apperror.Unauthorized("invalid or expired token"))
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
