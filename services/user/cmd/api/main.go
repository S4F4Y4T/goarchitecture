package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/grpcmiddleware"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	pb "github.com/s4f4y4t/go-microservice/pkg/proto/user"
	"github.com/s4f4y4t/go-microservice/services/user/internal/app"
	"github.com/s4f4y4t/go-microservice/services/user/internal/config"
	platformdatabase "github.com/s4f4y4t/go-microservice/services/user/internal/platform/database"
	userrouter "github.com/s4f4y4t/go-microservice/services/user/internal/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
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

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcmiddleware.RequestID,
			grpcmiddleware.Logger,
			grpcmiddleware.Recovery,
			grpcmiddleware.Validation,
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
	)
	pb.RegisterUserServiceServer(grpcServer, a.UserGRPCServer)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(pb.UserService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
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

	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	exitCode := 0

	wg.Add(2)
	go func() {
		defer wg.Done()
		// If GracefulStop hasn't finished by the deadline, force-stop it so
		// the process doesn't hang on a slow/stuck in-flight RPC.
		stopWaiting := context.AfterFunc(ctx, func() {
			slog.Warn("grpc graceful stop timed out, forcing stop")
			grpcServer.Stop()
		})
		grpcServer.GracefulStop()
		stopWaiting()
	}()
	go func() {
		defer wg.Done()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("server forced to shutdown", "error", err)
			exitCode = 1
		}
	}()
	wg.Wait()

	os.Exit(exitCode)
}
