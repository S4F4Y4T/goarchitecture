package middleweare

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"microservice/pkg/appError"
	"microservice/pkg/response"
)

func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[%s] panic %s %s: %v\n%s", GetRequestID(r.Context()), r.Method, r.URL.Path, rec, debug.Stack())
				response.Error(w, r, appError.Internal(fmt.Errorf("panic: %v", rec)))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
