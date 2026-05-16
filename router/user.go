package router

import (
	"microservice/internals/handler"
	"net/http"
)

func RegisterUsersRoute(mux *http.ServeMux, handler *handler.UserHandler) *http.ServeMux {
	userMux := http.NewServeMux()

	userMux.HandleFunc("GET /", handler.GetUserByID)
	userMux.HandleFunc("GET /{id}", handler.GetUserByID)
	userMux.HandleFunc("POST /", handler.GetUserByID)
	userMux.HandleFunc("PUT /{id}", handler.GetUserByID)
	userMux.HandleFunc("DELETE /{id}", handler.GetUserByID)

	mux.Handle("/users/", http.StripPrefix("/users", userMux))
	return mux
}
