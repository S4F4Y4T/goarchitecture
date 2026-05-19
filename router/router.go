package router

import (
	"microservice/internals/bootstrap"
	"microservice/internals/middleweare"
	"net/http"
)

func Register(handler *bootstrap.App) http.Handler {
	mux := http.NewServeMux()

	RegisterUsersRoute(mux, handler.UserHandler)

	return middleweare.Chain(middleweare.Logger, middleweare.Cors, middleweare.PanicRecovery)(mux)
}
