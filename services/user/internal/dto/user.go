package dto

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

// UpdateUserRequest is a full representation of the user: PUT replaces the
// entire resource, so every field is required.
type UpdateUserRequest struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type RegisterDTO struct {
	Name     string `json:"name"     validate:"required"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginDTO struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
