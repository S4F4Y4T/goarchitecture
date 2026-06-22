package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/logger"
	pb "github.com/s4f4y4t/go-microservice/pkg/proto/user"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/app"
	"github.com/s4f4y4t/go-microservice/services/auth/internal/config"
	platformredis "github.com/s4f4y4t/go-microservice/services/auth/internal/platform/redis"
	authrouter "github.com/s4f4y4t/go-microservice/services/auth/internal/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.Init(os.Stdout, logger.ParseLevel(os.Getenv("LOG_LEVEL")))

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	rdb, err := platformredis.Open(cfg.Redis)
	if err != nil {
		slog.Error("setting up redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	userConn, err := grpc.NewClient(cfg.UserGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("connecting to user grpc service", "error", err)
		os.Exit(1)
	}
	defer userConn.Close()
	userClient := pb.NewUserServiceClient(userConn)

	a := app.Build(userClient, rdb, token.NewRSAIssuer(cfg.JWT.PrivateKey), cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry, cfg.JWT.CookieSecure)

	mux := authrouter.Register(a.AuthHandler, a.HealthHandler)

	srv := &http.Server{
		Addr:           ":" + strconv.Itoa(cfg.Port),
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	slog.Info("shutting down server")

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
}
