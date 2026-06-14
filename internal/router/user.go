package router

import (
	"microservice/internal/handler"
	"net/http"
)

func RegisterUsersRoute(mux *http.ServeMux, handler *handler.UserHandler) *http.ServeMux {
	userMux := http.NewServeMux()

	userMux.HandleFunc("GET /", handler.GetAllUsers)
	userMux.HandleFunc("GET /{id}", handler.GetUserByID)
	userMux.HandleFunc("POST /", handler.CreateUser)
	userMux.HandleFunc("PUT /{id}", handler.UpdateUser)
	userMux.HandleFunc("DELETE /{id}", handler.DeleteUser)

	mux.Handle("/users/", http.StripPrefix("/users", userMux))
	return mux
}
