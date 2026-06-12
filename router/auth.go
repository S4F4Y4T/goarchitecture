package router

import (
	"microservice/internal/handler"
	"microservice/pkg/middleware"
	"net/http"
)

func RegisterAuthRoutes(mux *http.ServeMux, handler *handler.AuthHandler, auth middleware.Middleware) *http.ServeMux {
	authMux := http.NewServeMux()

	authMux.HandleFunc("POST /register", handler.Register)
	authMux.HandleFunc("POST /login", handler.Login)
	authMux.Handle("GET /me", middleware.With(handler.Me, auth))

	mux.Handle("/auth/", http.StripPrefix("/auth", authMux))
	return mux
}
