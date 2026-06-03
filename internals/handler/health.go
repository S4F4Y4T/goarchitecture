package handler

import (
	"context"
	"net/http"
	"time"

	"microservice/pkg/response"

	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live reports process liveness. It performs no dependency checks: if the
// process can serve this request, it is alive.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, "alive", nil)
}

// Ready reports readiness to serve traffic, verifying the database is
// reachable with a short-lived ping. It returns 503 when the DB is unavailable
// so orchestrators stop routing traffic to this instance.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		response.JSONResponse(w, http.StatusServiceUnavailable, response.ApiResponse{
			Success: false,
			Message: "database unreachable",
		})
		return
	}

	response.Success(w, http.StatusOK, "ready", nil)
}
