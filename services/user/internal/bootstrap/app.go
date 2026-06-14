package bootstrap

import (
	"microservice/services/user/internal/handler"
	"microservice/services/user/internal/repository"
	"microservice/services/user/internal/service"

	"gorm.io/gorm"
)

type App struct {
	UserHandler    *handler.UserHandler
	ProductHandler *handler.ProductHandler
	HealthHandler  *handler.HealthHandler
}

func Register(db *gorm.DB) *App {
	urepo := repository.NewUserRepository(db)
	uservice := service.NewUserService(urepo)
	uhandler := handler.NewUserHandler(uservice)

	prepo := repository.NewProductRepository(db)
	pservice := service.NewProductService(prepo)
	phandler := handler.NewProductHandler(pservice)

	hhandler := handler.NewHealthHandler(db)

	return &App{
		UserHandler:    uhandler,
		ProductHandler: phandler,
		HealthHandler:  hhandler,
	}
}
