// Package app is the global composition root for every services/user
// entry point (cmd/api today; cmd/grpc, cmd/worker, cmd/kafkaconsumer in
// the future). It is the only place that builds shared infrastructure
// (via internal/platform) and the only place that knows every module
// exists — modules themselves never open a DB/Redis connection and never
// know which entry point is using them.
package app

import (
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/user/internal/auth"
	"github.com/s4f4y4t/go-microservice/services/user/internal/health"
	"github.com/s4f4y4t/go-microservice/services/user/internal/user"
	"gorm.io/gorm"
)

// App holds only what each cmd/* entry point needs — not every
// intermediate repository/service built along the way.
type App struct {
	UserHTTPHandler *user.UserHandler
	AuthHTTPHandler *auth.AuthHandler
	HealthHandler   *health.Handler
}

// Build wires every module together. db and rdb are already-open
// connections built by internal/platform — Build never opens one itself.
func Build(db *gorm.DB, rdb *redis.Client, tokenIssuer token.AccessIssuer, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	userRepo := user.NewUserRepository(db)
	authRepo := auth.NewRepository(rdb)

	authSvc := auth.NewAuthService(userRepo, authRepo, tokenIssuer, accessExpiry, refreshExpiry)
	userSvc := user.NewUserService(userRepo)

	return &App{
		UserHTTPHandler: user.NewUserHandler(userSvc),
		AuthHTTPHandler: auth.NewAuthHandler(authSvc, cookieSecure),
		HealthHandler:   health.NewHandler(db),
	}
}
