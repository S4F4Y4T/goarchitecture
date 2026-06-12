package router

import (
	"microservice/config"
	"microservice/docs"
	"microservice/internal/bootstrap"
	"microservice/internal/model"
	"microservice/pkg/middleware"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

func Register(handler *bootstrap.App, cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	// Route-level auth middleware: auth rejects anonymous requests, admin
	// additionally requires the admin role (and must run after auth).
	auth := middleware.Authenticate(handler.Tokens)
	admin := middleware.RequireRole(model.RoleAdmin)

	// Versioned API: routes live under /v1/ so breaking changes can ship as
	// /v2/ without breaking existing clients.
	v1 := http.NewServeMux()
	RegisterAuthRoutes(v1, handler.AuthHandler, auth)
	RegisterUsersRoute(v1, handler.UserHandler, auth, admin)
	RegisterProductRoutes(v1, handler.ProductHandler, auth, admin)
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

	return middleware.Chain(middleware.RequestID, middleware.Logger, middleware.Cors(cfg.CORS.AllowedOrigins), middleware.PanicRecovery)(mux)
}
