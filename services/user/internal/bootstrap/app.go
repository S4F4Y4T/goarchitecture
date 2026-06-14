package bootstrap

import (
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

func Register(db *gorm.DB) *App {
	urepo := repository.NewUserRepository(db)
	uservice := service.NewUserService(urepo)
	uhandler := handler.NewUserHandler(uservice)

	ahandler := handler.NewAuthHandler(uservice)

	return &App{
		UserHandler:   uhandler,
		AuthHandler:   ahandler,
		HealthHandler: handler.NewHealthHandler(db),
	}
}
