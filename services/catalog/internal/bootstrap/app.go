package bootstrap

import (
	"microservice/services/catalog/internal/handler"
	"microservice/services/catalog/internal/repository"
	"microservice/services/catalog/internal/service"

	"gorm.io/gorm"
)

type App struct {
	ProductHandler *handler.ProductHandler
	HealthHandler  *handler.HealthHandler
}

func Register(db *gorm.DB) *App {
	prepo := repository.NewProductRepository(db)
	pservice := service.NewProductService(prepo)
	phandler := handler.NewProductHandler(pservice)

	return &App{
		ProductHandler: phandler,
		HealthHandler:  handler.NewHealthHandler(db),
	}
}
