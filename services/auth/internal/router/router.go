package router

import (
	"net/http"

	pkgmiddleware "github.com/s4f4y4t/go-microservice/pkg/middleware"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/auth"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/health"
)

func Register(authH *auth.Handler, healthH *health.Handler) http.Handler {
	mux := http.NewServeMux()

	v1 := http.NewServeMux()
	registerAuthRoutes(v1, authH)

	mux.Handle("/v1/", http.StripPrefix("/v1", v1))

	mux.HandleFunc("GET /healthz", healthH.Live)
	mux.HandleFunc("GET /readyz", healthH.Ready)

	return pkgmiddleware.Chain(
		pkgmiddleware.RequestID,
		pkgmiddleware.Logger,
		pkgmiddleware.PanicRecovery,
	)(mux)
}
