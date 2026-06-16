package bootstrap

import (
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/user/internal/cache"
	"github.com/s4f4y4t/go-microservice/services/user/internal/handler"
	"github.com/s4f4y4t/go-microservice/services/user/internal/repository"
	"github.com/s4f4y4t/go-microservice/services/user/internal/service"
	"gorm.io/gorm"
)

type App struct {
	UserHandler   *handler.UserHandler
	AuthHandler   *handler.AuthHandler
	HealthHandler *handler.HealthHandler
}

func Register(db *gorm.DB, rdb *redis.Client, tokenIssuer token.AccessIssuer, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	repo := repository.NewUserRepository(db)
	tokenStore := cache.NewRedisTokenStore(rdb)

	authSvc := service.NewAuthService(repo, tokenStore, tokenIssuer, accessExpiry, refreshExpiry)
	userSvc := service.NewUserService(repo)

	return &App{
		UserHandler:   handler.NewUserHandler(userSvc),
		AuthHandler:   handler.NewAuthHandler(authSvc, cookieSecure),
		HealthHandler: handler.NewHealthHandler(db),
	}
}
