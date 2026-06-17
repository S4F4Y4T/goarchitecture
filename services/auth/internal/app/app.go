package app

import (
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/auth"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/health"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/user"
	"gorm.io/gorm"
)

type App struct {
	AuthHandler   *auth.Handler
	HealthHandler *health.Handler
}

func Build(db *gorm.DB, rdb *redis.Client, tokenIssuer token.AccessIssuer, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	userRepo := user.NewRepository(db)
	tokenStore := auth.NewTokenRepository(rdb)

	authSvc := auth.NewService(userRepo, tokenStore, tokenIssuer, accessExpiry, refreshExpiry)

	return &App{
		AuthHandler:   auth.NewHandler(authSvc, cookieSecure),
		HealthHandler: health.NewHandler(db),
	}
}
