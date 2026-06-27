package router

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/services/notification/internal/notification"
)

func registerNotificationRoutes(mux *http.ServeMux, h *notification.NotificationHandler, auth func(http.Handler) http.Handler) {
	notificationMux := http.NewServeMux()

	notificationMux.HandleFunc("GET /", h.GetAll)
	notificationMux.HandleFunc("GET /{id}", h.GetByID)

	mux.Handle("/notifications/", auth(http.StripPrefix("/notifications", notificationMux)))
}
