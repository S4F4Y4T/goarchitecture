package bootstrap

import (
	"microservice/config"
	"microservice/internal/handler"
	"microservice/internal/repository"
	"microservice/internal/service"
	"microservice/pkg/token"

	"gorm.io/gorm"
)

type App struct {
	UserHandler    *handler.UserHandler
	ProductHandler *handler.ProductHandler
	HealthHandler  *handler.HealthHandler
	AuthHandler    *handler.AuthHandler

	// Tokens verifies access tokens; the router uses it to build the
	// authentication middleware.
	Tokens *token.Manager
}

func Register(db *gorm.DB, cfg *config.Config) *App {
	tokens := token.NewManager(cfg.Auth.JWTSecret, cfg.Auth.JWTTTL)

	urepo := repository.NewUserRepository(db)
	uservice := service.NewUserService(urepo)
	uhandler := handler.NewUserHandler(uservice)

	prepo := repository.NewProductRepository(db)
	pservice := service.NewProductService(prepo)
	phandler := handler.NewProductHandler(pservice)

	hhandler := handler.NewHealthHandler(db)

	aservice := service.NewAuthService(urepo, tokens)
	ahandler := handler.NewAuthHandler(aservice, uservice)

	return &App{
		UserHandler:    uhandler,
		ProductHandler: phandler,
		HealthHandler:  hhandler,
		AuthHandler:    ahandler,
		Tokens:         tokens,
	}
}
