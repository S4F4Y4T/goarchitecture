package bootstrap

import (
	"crypto/rsa"
	"time"

	"github.com/redis/go-redis/v9"
	deliveryhandler "github.com/s4f4y4t/go-microservice/services/user/internal/delivery/http/handler"
	"github.com/s4f4y4t/go-microservice/services/user/internal/infrastructure/cache"
	"github.com/s4f4y4t/go-microservice/services/user/internal/infrastructure/persistence"
	"github.com/s4f4y4t/go-microservice/services/user/internal/usecase"
	"gorm.io/gorm"
)

type App struct {
	UserHandler   *deliveryhandler.UserHandler
	AuthHandler   *deliveryhandler.AuthHandler
	HealthHandler *deliveryhandler.HealthHandler
}

func Register(db *gorm.DB, rdb *redis.Client, privateKey *rsa.PrivateKey, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	repo := persistence.NewUserRepository(db)
	tokenStore := cache.NewRedisTokenStore(rdb)

	authUC := usecase.NewAuthService(repo, tokenStore, privateKey, accessExpiry, refreshExpiry)
	userUC := usecase.NewUserService(repo)

	return &App{
		UserHandler:   deliveryhandler.NewUserHandler(userUC),
		AuthHandler:   deliveryhandler.NewAuthHandler(authUC, cookieSecure),
		HealthHandler: deliveryhandler.NewHealthHandler(db),
	}
}
