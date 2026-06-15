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
	"github.com/s4f4y4t/go-microservice/services/user/internal/bootstrap"
	"github.com/s4f4y4t/go-microservice/services/user/internal/config"
	"github.com/s4f4y4t/go-microservice/services/user/internal/router"
)

func main() {
	logger.Init(os.Stdout, logger.ParseLevel(os.Getenv("LOG_LEVEL")))

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	db, err := config.SetupDatabase(cfg.DB)
	if err != nil {
		slog.Error("setting up database", "error", err)
		os.Exit(1)
	}

	rdb, err := config.SetupRedis(cfg.Redis)
	if err != nil {
		slog.Error("setting up redis", "error", err)
		os.Exit(1)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	handler := bootstrap.Register(db, rdb, cfg.JWT.PrivateKey, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry, cfg.JWT.CookieSecure)

	mux := router.Register(handler, cfg, rdb)

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
