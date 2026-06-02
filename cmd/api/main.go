package main

import (
	"context"
	"fmt"
	"microservice/config"
	"microservice/internals/bootstrap"
	"microservice/router"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	db, err := config.SetupDatabase(cfg.DB)
	if err != nil {
		fmt.Printf("Error setting up database: %v\n", err)
		os.Exit(1)
	}

	handler := bootstrap.Register(db)

	mux := router.Register(handler)

	srv := &http.Server{
		Addr:           ":" + strconv.Itoa(cfg.Port),
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	fmt.Println("Shutting down server...")

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Server forced to shutdown: %v\n", err)
		os.Exit(1)
	}
}
