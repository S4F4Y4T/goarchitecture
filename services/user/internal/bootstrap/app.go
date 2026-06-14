package bootstrap

import (
	"github.com/s4f4y4t/go-microservice/services/user/internal/handler"
	"github.com/s4f4y4t/go-microservice/services/user/internal/repository"
	"github.com/s4f4y4t/go-microservice/services/user/internal/service"

	"gorm.io/gorm"
)

type App struct {
	UserHandler   *handler.UserHandler
	HealthHandler *handler.HealthHandler
}

func Register(db *gorm.DB) *App {
	urepo := repository.NewUserRepository(db)
	uservice := service.NewUserService(urepo)
	uhandler := handler.NewUserHandler(uservice)

	return &App{
		UserHandler:   uhandler,
		HealthHandler: handler.NewHealthHandler(db),
	}
}
