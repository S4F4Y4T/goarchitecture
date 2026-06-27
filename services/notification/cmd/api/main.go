package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	eventsuser "github.com/s4f4y4t/go-microservice/pkg/events/user"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"github.com/s4f4y4t/go-microservice/pkg/mailer"
	rabbitmqpkg "github.com/s4f4y4t/go-microservice/pkg/messaging/rabbitmq"
	"github.com/s4f4y4t/go-microservice/services/notification/internal/app"
	"github.com/s4f4y4t/go-microservice/services/notification/internal/config"
	platformdatabase "github.com/s4f4y4t/go-microservice/services/notification/internal/platform/database"
	platformrabbitmq "github.com/s4f4y4t/go-microservice/services/notification/internal/platform/rabbitmq"
	notificationrouter "github.com/s4f4y4t/go-microservice/services/notification/internal/router"
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

	amqpConn, err := platformrabbitmq.Open(cfg.RabbitMQ)
	if err != nil {
		slog.Error("setting up rabbitmq", "error", err)
		os.Exit(1)
	}

	sender := mailer.NewSMTPSender(
		cfg.SMTP.Host, cfg.SMTP.Port,
		cfg.SMTP.Username, cfg.SMTP.Password,
		cfg.SMTP.FromName, cfg.SMTP.FromAddress,
		cfg.SMTP.UseTLS,
	)

	a := app.Build(db, sender)

	// Declare the retry/DLQ topology on a short-lived channel before
	// starting the long-lived consumer — both the publisher (user service)
	// and this consumer declare the same exchange/queues idempotently, so
	// neither side has to assume the other started first.
	topologyCh, err := amqpConn.Channel()
	if err != nil {
		slog.Error("opening topology channel", "error", err)
		os.Exit(1)
	}
	if err := rabbitmqpkg.DeclareConsumerTopology(topologyCh, rabbitmqpkg.TopologySpec{
		Exchange:   cfg.RabbitMQ.Exchange,
		RoutingKey: cfg.Consumer.RoutingKey,
		QueueName:  cfg.Consumer.QueueName,
		RetryTTL:   cfg.Consumer.RetryTTL,
		MaxRetries: cfg.Consumer.MaxRetries,
	}); err != nil {
		slog.Error("declaring consumer topology", "error", err)
		os.Exit(1)
	}
	if err := topologyCh.Close(); err != nil {
		slog.Error("closing topology channel", "error", err)
	}

	consumer, err := rabbitmqpkg.NewConsumer(amqpConn)
	if err != nil {
		slog.Error("setting up consumer", "error", err)
		os.Exit(1)
	}

	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		slog.Info("consumer listening", "queue", cfg.Consumer.QueueName)
		if err := consumer.Consume(consumerCtx, cfg.Consumer.QueueName, cfg.Consumer.MaxRetries, handleUserCreated(a), recordExhausted(a)); err != nil {
			slog.Error("consumer stopped", "error", err)
		}
	}()

	mux := notificationrouter.Register(a.NotificationHTTPHandler, a.HealthHandler)

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
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
		cancelConsumer()
		select {
		case <-consumerDone:
		case <-ctx.Done():
			slog.Warn("consumer shutdown timed out")
		}
	}()
	go func() {
		defer wg.Done()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("server forced to shutdown", "error", err)
			exitCode = 1
		}
	}()
	wg.Wait()

	if err := amqpConn.Close(); err != nil {
		slog.Error("closing rabbitmq connection", "error", err)
	}

	os.Exit(exitCode)
}

func handleUserCreated(a *app.App) rabbitmqpkg.Handler {
	return func(ctx context.Context, env rabbitmqpkg.Envelope) error {
		var payload eventsuser.UserCreatedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return fmt.Errorf("decoding %s payload: %w", env.EventType, err)
		}
		return a.NotificationService.HandleUserCreated(ctx, env.EventID, payload)
	}
}

func recordExhausted(a *app.App) rabbitmqpkg.OnExhausted {
	return func(ctx context.Context, env rabbitmqpkg.Envelope, lastErr error) {
		var payload eventsuser.UserCreatedPayload
		_ = json.Unmarshal(env.Payload, &payload)
		if err := a.NotificationService.RecordFailure(ctx, env.EventID, env.EventType, payload.Email, lastErr.Error()); err != nil {
			slog.Error("recording exhausted notification failure", "event_id", env.EventID, "error", err)
		}
	}
}
