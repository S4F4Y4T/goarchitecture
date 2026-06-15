package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/services/user/internal/handler"
)

func RegisterAuthRoute(mux *http.ServeMux, h *handler.AuthHandler) *http.ServeMux {
	authMux := http.NewServeMux()

	authMux.HandleFunc("POST /register", h.Register)
	authMux.HandleFunc("POST /login", h.Login)
	authMux.HandleFunc("POST /refresh", h.Refresh)
	authMux.HandleFunc("POST /logout", h.Logout)

	mux.Handle("/auth/", http.StripPrefix("/auth", authMux))
	return mux
}
