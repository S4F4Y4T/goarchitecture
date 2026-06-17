package app

import (
	"github.com/s4f4y4t/go-microservice/services/user/internal/health"
	"github.com/s4f4y4t/go-microservice/services/user/internal/user"
	"gorm.io/gorm"
)

type App struct {
	UserHTTPHandler *user.UserHandler
	HealthHandler   *health.Handler
}

func Build(db *gorm.DB) *App {
	userRepo := user.NewUserRepository(db)
	userSvc := user.NewUserService(userRepo)

	return &App{
		UserHTTPHandler: user.NewUserHandler(userSvc),
		HealthHandler:   health.NewHandler(db),
	}
}
