package notification

import (
	"net/http"
	"strconv"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	"github.com/s4f4y4t/go-microservice/pkg/response"
)

// NotificationHandler is read-only: rows are written only by the RabbitMQ
// consumer (see cmd/api/main.go), this exposes them for ops/audit purposes.
type NotificationHandler struct {
	service *NotificationService
}

func NewNotificationHandler(service *NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()

	page, _ := strconv.Atoi(queryParams.Get("page"))
	limit, _ := strconv.Atoi(queryParams.Get("limit"))
	params := pagination.NewParams(page, limit)
	opts := query.Parse(queryParams, ListSchema)

	logger.FromContext(r.Context()).Debug("fetching notifications", "page", params.Page, "limit", params.Limit)

	notifications, total, err := h.service.GetAll(r.Context(), params, opts)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.SuccessWithMeta(w, http.StatusOK, "Notifications retrieved successfully", notifications, pagination.NewMeta(params, total))
}

func (h *NotificationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid notification id"))
		return
	}
	n, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "Notification retrieved successfully", n)
}
