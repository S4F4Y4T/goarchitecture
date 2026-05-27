package router

import (
	"microservice/docs"
	"microservice/internals/bootstrap"
	"microservice/internals/middleweare"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

func Register(handler *bootstrap.App) http.Handler {
	mux := http.NewServeMux()

	RegisterUsersRoute(mux, handler.UserHandler)

	mux.HandleFunc("GET /swagger/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(docs.SpecYAML)
	})
	mux.Handle("/swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/openapi.yaml")))

	return middleweare.Chain(middleweare.Logger, middleweare.Cors, middleweare.PanicRecovery)(mux)
}
