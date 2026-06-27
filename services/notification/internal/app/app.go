package app

import (
	"github.com/s4f4y4t/go-microservice/pkg/mailer"
	"github.com/s4f4y4t/go-microservice/services/notification/internal/health"
	"github.com/s4f4y4t/go-microservice/services/notification/internal/notification"
	"gorm.io/gorm"
)

type App struct {
	NotificationHTTPHandler *notification.NotificationHandler
	NotificationService     *notification.NotificationService
	HealthHandler           *health.Handler
}

func Build(db *gorm.DB, sender mailer.Sender) *App {
	notificationRepo := notification.NewNotificationRepository(db)
	notificationSvc := notification.NewNotificationService(notificationRepo, sender)

	return &App{
		NotificationHTTPHandler: notification.NewNotificationHandler(notificationSvc),
		NotificationService:     notificationSvc,
		HealthHandler:           health.NewHandler(db),
	}
}
