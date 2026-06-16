package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/services/user/internal/auth"
)

func registerAuthRoutes(mux *http.ServeMux, h *auth.AuthHandler) {
	authMux := http.NewServeMux()

	authMux.HandleFunc("POST /register", h.Register)
	authMux.HandleFunc("POST /login", h.Login)
	authMux.HandleFunc("POST /refresh", h.Refresh)
	authMux.HandleFunc("POST /logout", h.Logout)

	mux.Handle("/auth/", http.StripPrefix("/auth", authMux))
}
