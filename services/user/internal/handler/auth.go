package handler

import (
	"net/http"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/request"
	"github.com/s4f4y4t/go-microservice/pkg/response"
	"github.com/s4f4y4t/go-microservice/pkg/validation"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/service"
	"github.com/s4f4y4t/go-microservice/services/user/internal/token"
)

type AuthHandler struct {
	service    *service.UserService
	jwtSecret  string
	jwtExpiry  time.Duration
}

func NewAuthHandler(svc *service.UserService, jwtSecret string, jwtExpiry time.Duration) *AuthHandler {
	return &AuthHandler{service: svc, jwtSecret: jwtSecret, jwtExpiry: jwtExpiry}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterDTO
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	user, err := h.service.Register(r.Context(), req)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusCreated, "User registered successfully", user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginDTO
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	user, err := h.service.Login(r.Context(), req)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	t, err := token.Generate(user.ID, h.jwtSecret, h.jwtExpiry)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "Login successful", map[string]string{"token": t})
}
