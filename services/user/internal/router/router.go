package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/pkg/middleware"
	"github.com/s4f4y4t/go-microservice/services/user/docs"
	"github.com/s4f4y4t/go-microservice/services/user/internal/bootstrap"
	"github.com/s4f4y4t/go-microservice/services/user/internal/config"
	svcmiddleware "github.com/s4f4y4t/go-microservice/services/user/internal/middleware"

	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
)

func Register(handler *bootstrap.App, cfg *config.Config, rdb *redis.Client) http.Handler {
	mux := http.NewServeMux()

	v1 := http.NewServeMux()
	RegisterUsersRoute(v1, handler.UserHandler, svcmiddleware.Auth(cfg.JWT.Secret))
	RegisterAuthRoute(v1, handler.AuthHandler)

	mux.Handle("/v1/", http.StripPrefix("/v1", v1))

	mux.HandleFunc("GET /healthz", handler.HealthHandler.Live)
	mux.HandleFunc("GET /readyz", handler.HealthHandler.Ready)

	mux.HandleFunc("GET /swagger/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(docs.SpecYAML)
	})
	mux.Handle("/swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/openapi.yaml")))

	return middleware.Chain(
		middleware.RequestID,
		middleware.Logger,
		// middleware.Cors(cfg.CORS.AllowedOrigins),        // handled by Kong
		// middleware.RateLimit(rdb, "user", cfg.RateLimit.Requests, cfg.RateLimit.Window), // handled by Kong
		middleware.PanicRecovery,
	)(mux)
}
