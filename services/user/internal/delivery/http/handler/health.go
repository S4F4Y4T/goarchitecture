package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/response"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, "alive", nil)
}

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
