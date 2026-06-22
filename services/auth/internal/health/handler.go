package health

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s4f4y4t/go-microservice/pkg/response"
)

type Handler struct {
	rdb *redis.Client
}

func NewHandler(rdb *redis.Client) *Handler {
	return &Handler{rdb: rdb}
}

func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, "alive", nil)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.rdb.Ping(ctx).Err(); err != nil {
		response.JSONResponse(w, http.StatusServiceUnavailable, response.ApiResponse{
			Success: false,
			Message: "redis unreachable",
		})
		return
	}

	response.Success(w, http.StatusOK, "ready", nil)
}
