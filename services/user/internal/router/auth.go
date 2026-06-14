package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/services/user/internal/handler"
)

func RegisterAuthRoute(mux *http.ServeMux, handler *handler.AuthHandler) *http.ServeMux {
	authM := http.NewServeMux()

	authM.HandleFunc("POST /login", handler.Login)
	authM.HandleFunc("POST /register", handler.Register)

	mux.Handle("/auth/", http.StripPrefix("/auth", authM))
	return mux
}
