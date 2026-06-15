package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/request"
	"github.com/s4f4y4t/go-microservice/pkg/response"
	"github.com/s4f4y4t/go-microservice/pkg/validation"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/service"
	"github.com/s4f4y4t/go-microservice/pkg/token"
)

type AuthHandler struct {
	service       *service.UserService
	tokenStore    token.Store
	jwtSecret     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewAuthHandler(
	svc *service.UserService,
	store token.Store,
	jwtSecret string,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
) *AuthHandler {
	return &AuthHandler{
		service:       svc,
		tokenStore:    store,
		jwtSecret:     jwtSecret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
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

	tokens, err := h.issueTokenPair(r, user.ID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "Login successful", tokens)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	userID, err := h.tokenStore.UserID(r.Context(), req.RefreshToken)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	// Rotate: delete the used token before issuing a new pair.
	if err := h.tokenStore.Delete(r.Context(), req.RefreshToken); err != nil {
		response.Error(w, r, apperror.Internal(err))
		return
	}

	tokens, err := h.issueTokenPair(r, userID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "Token refreshed", tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	// Best-effort: ignore "not found" so logout is idempotent.
	_ = h.tokenStore.Delete(r.Context(), req.RefreshToken)

	response.NoContent(w)
}

// issueTokenPair generates a fresh access JWT and a new UUID refresh token,
// persists the refresh token, and returns both as a map ready for the response.
func (h *AuthHandler) issueTokenPair(r *http.Request, userID int) (map[string]any, error) {
	accessToken, err := token.Generate(userID, h.jwtSecret, h.accessExpiry)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	refreshToken := uuid.NewString()
	if err := h.tokenStore.Save(r.Context(), refreshToken, userID, h.refreshExpiry); err != nil {
		return nil, apperror.Internal(err)
	}

	return map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.accessExpiry.Seconds()),
	}, nil
}
