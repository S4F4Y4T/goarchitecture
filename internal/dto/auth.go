package dto

import "microservice/internal/model"

type RegisterRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse is returned by register and login: the issued access token plus
// the authenticated user.
type AuthResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}
