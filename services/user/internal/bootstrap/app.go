package bootstrap

import (
	"time"

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

func Register(db *gorm.DB, jwtSecret string, jwtExpiry time.Duration) *App {
	urepo := repository.NewUserRepository(db)
	uservice := service.NewUserService(urepo)

	return &App{
		UserHandler:   handler.NewUserHandler(uservice),
		AuthHandler:   handler.NewAuthHandler(uservice, jwtSecret, jwtExpiry),
		HealthHandler: handler.NewHealthHandler(db),
	}
}
