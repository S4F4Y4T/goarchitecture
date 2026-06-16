package bootstrap

import (
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/user/internal/auth"
	"github.com/s4f4y4t/go-microservice/services/user/internal/health"
	"github.com/s4f4y4t/go-microservice/services/user/internal/user"
	"gorm.io/gorm"
)

type App struct {
	UserHandler   *user.UserHandler
	AuthHandler   *auth.AuthHandler
	HealthHandler *health.Handler
}

func Register(db *gorm.DB, rdb *redis.Client, tokenIssuer token.AccessIssuer, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	repo := user.NewUserRepository(db)
	tokenStore := auth.NewRedisTokenStore(rdb)

	authSvc := auth.NewAuthService(repo, tokenStore, tokenIssuer, accessExpiry, refreshExpiry)
	userSvc := user.NewUserService(repo)

	return &App{
		UserHandler:   user.NewUserHandler(userSvc),
		AuthHandler:   auth.NewAuthHandler(authSvc, cookieSecure),
		HealthHandler: health.NewHandler(db),
	}
}
