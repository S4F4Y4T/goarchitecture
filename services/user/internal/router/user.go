package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/services/user/internal/user"
)

func registerUserRoutes(mux *http.ServeMux, h *user.UserHandler, auth func(http.Handler) http.Handler) {
	userMux := http.NewServeMux()

	userMux.HandleFunc("GET /", h.GetAll)
	userMux.HandleFunc("GET /{id}", h.GetByID)
	userMux.HandleFunc("POST /", h.Create)
	userMux.HandleFunc("PUT /{id}", h.Update)
	userMux.HandleFunc("DELETE /{id}", h.Delete)

	mux.Handle("/users/", auth(http.StripPrefix("/users", userMux)))
}
