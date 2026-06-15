package handler

import (
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/request"
	"github.com/s4f4y4t/go-microservice/pkg/response"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/pkg/validation"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/service"
)

const refreshCookieName = "refresh_token"

type AuthHandler struct {
	service       *service.AuthService
	tokenStore    token.Store
	privateKey    *rsa.PrivateKey
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	cookieSecure  bool
}

func NewAuthHandler(
	svc *service.AuthService,
	store token.Store,
	privateKey *rsa.PrivateKey,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
	cookieSecure bool,
) *AuthHandler {
	return &AuthHandler{
		service:       svc,
		tokenStore:    store,
		privateKey:    privateKey,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
		cookieSecure:  cookieSecure,
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

	tokens, err := h.issueTokenPair(w, r, user.ID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "Login successful", tokens)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		response.Error(w, r, apperror.Unauthorized("missing refresh token"))
		return
	}

	userID, err := h.tokenStore.UserID(r.Context(), cookie.Value)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	// Rotate: delete the used token before issuing a new pair.
	if err := h.tokenStore.Delete(r.Context(), cookie.Value); err != nil {
		response.Error(w, r, apperror.Internal(err))
		return
	}

	tokens, err := h.issueTokenPair(w, r, userID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "Token refreshed", tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err == nil {
		// Best-effort: ignore "not found" so logout is idempotent.
		_ = h.tokenStore.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/v1/auth",
		MaxAge:   -1,
	})

	response.NoContent(w)
}

// issueTokenPair generates a JWT access token and a UUID refresh token,
// persists the refresh token in Redis, sets it as an httpOnly cookie,
// and returns the access token + expiry for the response body.
func (h *AuthHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, userID int) (map[string]any, error) {
	accessToken, err := token.Generate(userID, h.privateKey, h.accessExpiry)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	refreshToken := uuid.NewString()
	if err := h.tokenStore.Save(r.Context(), refreshToken, userID, h.refreshExpiry); err != nil {
		return nil, apperror.Internal(err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/v1/auth",
		MaxAge:   int(h.refreshExpiry.Seconds()),
	})

	return map[string]any{
		"access_token": accessToken,
		"expires_in":   int(h.accessExpiry.Seconds()),
	}, nil
}
