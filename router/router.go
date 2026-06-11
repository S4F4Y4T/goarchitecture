package router

import (
	"microservice/docs"
	"microservice/internal/bootstrap"
	"microservice/pkg/middleware"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

func Register(handler *bootstrap.App) http.Handler {
	mux := http.NewServeMux()

	// Versioned API: routes live under /v1/ so breaking changes can ship as
	// /v2/ without breaking existing clients.
	v1 := http.NewServeMux()
	RegisterUsersRoute(v1, handler.UserHandler)
	RegisterProductRoutes(v1, handler.ProductHandler)
	mux.Handle("/v1/", http.StripPrefix("/v1", v1))

	// Health endpoints stay unversioned at the root so container orchestrators
	// can rely on stable probe paths.
	mux.HandleFunc("GET /healthz", handler.HealthHandler.Live)
	mux.HandleFunc("GET /readyz", handler.HealthHandler.Ready)

	mux.HandleFunc("GET /swagger/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(docs.SpecYAML)
	})
	mux.Handle("/swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/openapi.yaml")))

	return middleware.Chain(middleware.RequestID, middleware.Logger, middleware.Cors, middleware.PanicRecovery)(mux)
}
