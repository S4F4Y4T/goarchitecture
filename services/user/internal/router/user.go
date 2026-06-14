package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/services/user/internal/handler"
)

func RegisterUsersRoute(mux *http.ServeMux, h *handler.UserHandler, auth func(http.Handler) http.Handler) *http.ServeMux {
	userMux := http.NewServeMux()

	userMux.HandleFunc("GET /", h.GetAllUsers)
	userMux.HandleFunc("GET /{id}", h.GetUserByID)
	userMux.HandleFunc("POST /", h.CreateUser)
	userMux.HandleFunc("PUT /{id}", h.UpdateUser)
	userMux.HandleFunc("DELETE /{id}", h.DeleteUser)

	mux.Handle("/users/", auth(http.StripPrefix("/users", userMux)))
	return mux
}
