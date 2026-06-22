package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/logger"
	pb "github.com/s4f4y4t/go-microservice/pkg/proto/user"
	"github.com/s4f4y4t/go-microservice/services/user/internal/app"
	"github.com/s4f4y4t/go-microservice/services/user/internal/config"
	platformdatabase "github.com/s4f4y4t/go-microservice/services/user/internal/platform/database"
	userrouter "github.com/s4f4y4t/go-microservice/services/user/internal/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	logger.Init(os.Stdout, logger.ParseLevel(os.Getenv("LOG_LEVEL")))

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	db, err := platformdatabase.Open(cfg.DB)
	if err != nil {
		slog.Error("setting up database", "error", err)
		os.Exit(1)
	}

	a := app.Build(db)

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(recoveryInterceptor))
	pb.RegisterUserServiceServer(grpcServer, a.UserGRPCServer)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.GRPCPort))
	if err != nil {
		slog.Error("grpc listen", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("grpc server listening", "addr", lis.Addr().String())
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("grpc server error", "error", err)
			os.Exit(1)
		}
	}()

	mux := userrouter.Register(a.UserHTTPHandler, a.HealthHandler)

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

	grpcServer.GracefulStop()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
}

// recoveryInterceptor turns a panic in an RPC handler into an Internal error
// instead of crashing the process, mirroring pkgmiddleware.PanicRecovery on
// the HTTP side.
func recoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("grpc handler panic", "method", info.FullMethod, "panic", r)
			err = status.Error(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}
