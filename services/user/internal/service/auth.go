package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/model"
)

type AuthService struct {
	repo model.UserRepository
}

func NewAuthService(repo model.UserRepository) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) Register(c context.Context, req dto.RegisterDTO) (*model.User, error) {
	exists, err := s.repo.ExistsByEmail(c, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("email already exists")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	return s.repo.CreateUser(c, &model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashed),
	})
}

func (s *AuthService) Login(c context.Context, req dto.LoginDTO) (*model.User, error) {
	user, err := s.repo.GetByEmail(c, req.Email)
	if err != nil {
		return nil, apperror.Unauthorized("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, apperror.Unauthorized("invalid email or password")
	}

	return user, nil
}
