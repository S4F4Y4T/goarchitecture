package handler

import (
	"net/http"

	"microservice/internal/dto"
	"microservice/internal/service"
	"microservice/pkg/apperror"
	"microservice/pkg/logger"
	"microservice/pkg/middleware"
	"microservice/pkg/request"
	"microservice/pkg/response"
	"microservice/pkg/validation"
)

type AuthHandler struct {
	auth  *service.AuthService
	users *service.UserService
}

func NewAuthHandler(auth *service.AuthService, users *service.UserService) *AuthHandler {
	return &AuthHandler{auth: auth, users: users}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest

	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	logger.FromContext(r.Context()).Info("registering user", "email", req.Email)

	res, err := h.auth.Register(r.Context(), req)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, "User registered successfully", res)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest

	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	res, err := h.auth.Login(r.Context(), req)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "Logged in successfully", res)
}

// Me returns the authenticated caller's own user record, looked up fresh so
// it reflects changes made after the token was issued.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.Claims(r.Context())
	if claims == nil {
		response.Error(w, r, apperror.Unauthorized("not authenticated"))
		return
	}

	user, err := h.users.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "User retrieved successfully", user)
}
