package router

import (
	"microservice/internals/bootstrap"
	"net/http"
)

func Register(handler *bootstrap.App) *http.ServeMux {
	mux := http.NewServeMux()

	RegisterUsersRoute(mux, handler.UserHandler)

	return mux
}
