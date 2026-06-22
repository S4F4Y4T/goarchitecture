package app

import (
	"time"

	"github.com/redis/go-redis/v9"
	pb "github.com/s4f4y4t/go-microservice/pkg/proto/user"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/auth"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/health"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/user"
)

type App struct {
	AuthHandler   *auth.Handler
	HealthHandler *health.Handler
}

func Build(userClient pb.UserServiceClient, rdb *redis.Client, tokenIssuer token.AccessIssuer, accessExpiry, refreshExpiry time.Duration, cookieSecure bool) *App {
	userLookup := user.NewClient(userClient)
	tokenStore := auth.NewTokenRepository(rdb)

	authSvc := auth.NewService(userLookup, tokenStore, tokenIssuer, accessExpiry, refreshExpiry)

	return &App{
		AuthHandler:   auth.NewHandler(authSvc, cookieSecure),
		HealthHandler: health.NewHandler(rdb),
	}
}
