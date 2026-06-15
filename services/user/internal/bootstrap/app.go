package bootstrap

import (
	"time"

	"github.com/redis/go-redis/v9"
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

func Register(db *gorm.DB, rdb *redis.Client, jwtSecret string, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	urepo := repository.NewUserRepository(db)
	uservice := service.NewUserService(urepo)
	aservice := service.NewAuthService(urepo)
	tokenStore := repository.NewRedisTokenStore(rdb)

	return &App{
		UserHandler:   handler.NewUserHandler(uservice),
		AuthHandler:   handler.NewAuthHandler(aservice, tokenStore, jwtSecret, accessExpiry, refreshExpiry, cookieSecure),
		HealthHandler: handler.NewHealthHandler(db),
	}
}
