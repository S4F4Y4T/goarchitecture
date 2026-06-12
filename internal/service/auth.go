package service

import (
	"context"
	"errors"

	"microservice/internal/dto"
	"microservice/internal/model"
	"microservice/pkg/apperror"
	"microservice/pkg/token"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo   model.UserRepository
	tokens *token.Manager
}

func NewAuthService(repo model.UserRepository, tokens *token.Manager) *AuthService {
	return &AuthService{repo: repo, tokens: tokens}
}

// Register creates a new account with the "user" role and logs it in. Roles
// are never taken from client input; promoting to admin is a manual operation.
func (s *AuthService) Register(c context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	exists, err := s.repo.ExistsByEmail(c, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("email already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	user, err := s.repo.CreateUser(c, &model.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         model.RoleUser,
	})
	if err != nil {
		return nil, err
	}

	return s.issue(user)
}

// Login verifies credentials and issues a token. Wrong email and wrong
// password return the same Unauthorized error so callers can't probe which
// emails have accounts.
func (s *AuthService) Login(c context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	invalid := apperror.Unauthorized("invalid email or password")

	user, err := s.repo.GetUserByEmail(c, req.Email)
	if err != nil {
		var appErr *apperror.AppError
		if errors.As(err, &appErr) && appErr.Code == apperror.CodeNotFound {
			return nil, invalid
		}
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, invalid
	}

	return s.issue(user)
}

func (s *AuthService) issue(user *model.User) (*dto.AuthResponse, error) {
	t, err := s.tokens.Sign(user.ID, user.Role)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &dto.AuthResponse{Token: t, User: user}, nil
}
