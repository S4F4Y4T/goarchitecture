package bootstrap

import (
	"github.com/s4f4y4t/go-microservice/services/catalog/internal/handler"
	"github.com/s4f4y4t/go-microservice/services/catalog/internal/repository"
	"github.com/s4f4y4t/go-microservice/services/catalog/internal/service"

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
