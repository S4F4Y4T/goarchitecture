package port

import (
	"context"

	userDomain "github.com/s4f4y4t/go-microservice/services/user/internal/domain/user"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  int
	RefreshExpiresIn int
}

type AuthUseCase interface {
	Register(ctx context.Context, input RegisterInput) (*userDomain.User, error)
	Login(ctx context.Context, input LoginInput) (TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}
