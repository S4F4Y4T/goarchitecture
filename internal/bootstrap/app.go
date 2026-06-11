package bootstrap

import (
	"microservice/internals/handler"
	"microservice/internals/repository"
	"microservice/internals/service"

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
