package router

import (
	"net/http"

	"github.com/redis/go-redis/v9"
	pkgmiddleware "github.com/s4f4y4t/go-microservice/pkg/middleware"
	"github.com/s4f4y4t/go-microservice/services/user/internal/config"
	"github.com/s4f4y4t/go-microservice/services/user/internal/handler"
)

func Register(
	userH *handler.UserHandler,
	authH *handler.AuthHandler,
	healthH *handler.HealthHandler,
	cfg *config.Config,
	rdb *redis.Client,
) http.Handler {
	mux := http.NewServeMux()

	v1 := http.NewServeMux()
	registerUserRoutes(v1, userH, pkgmiddleware.Auth())
	registerAuthRoutes(v1, authH)

	mux.Handle("/v1/", http.StripPrefix("/v1", v1))

	mux.HandleFunc("GET /healthz", healthH.Live)
	mux.HandleFunc("GET /readyz", healthH.Ready)

	return pkgmiddleware.Chain(
		pkgmiddleware.RequestID,
		pkgmiddleware.Logger,
		// pkgmiddleware.Cors(cfg.CORS.AllowedOrigins),        // handled by Kong
		// pkgmiddleware.RateLimit(rdb, "user", cfg.RateLimit.Requests, cfg.RateLimit.Window), // handled by Kong
		pkgmiddleware.PanicRecovery,
	)(mux)
}
