package handler

import (
	"net/http"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/request"
	"github.com/s4f4y4t/go-microservice/pkg/response"
	"github.com/s4f4y4t/go-microservice/pkg/validation"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/usecase/port"
)

const refreshCookieName = "refresh_token"

type AuthHandler struct {
	useCase      port.AuthUseCase
	cookieSecure bool
}

func NewAuthHandler(uc port.AuthUseCase, cookieSecure bool) *AuthHandler {
	return &AuthHandler{useCase: uc, cookieSecure: cookieSecure}
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

	user, err := h.useCase.Register(r.Context(), port.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
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

	pair, err := h.useCase.Login(r.Context(), port.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.Error(w, r, err)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken, pair.RefreshExpiresIn)
	response.Success(w, http.StatusOK, "Login successful", map[string]any{
		"access_token": pair.AccessToken,
		"expires_in":   pair.AccessExpiresIn,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		response.Error(w, r, apperror.Unauthorized("missing refresh token"))
		return
	}

	pair, err := h.useCase.Refresh(r.Context(), cookie.Value)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken, pair.RefreshExpiresIn)
	response.Success(w, http.StatusOK, "Token refreshed", map[string]any{
		"access_token": pair.AccessToken,
		"expires_in":   pair.AccessExpiresIn,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err == nil {
		_ = h.useCase.Logout(r.Context(), cookie.Value)
	}

	h.setRefreshCookie(w, "", -1)
	response.NoContent(w)
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/v1/auth",
		MaxAge:   maxAge,
	})
}
