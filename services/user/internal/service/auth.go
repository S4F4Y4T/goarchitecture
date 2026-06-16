package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/model"
)

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  int
	RefreshExpiresIn int
}

type AuthService struct {
	repo          model.Repository
	tokenStore    token.Store
	tokenIssuer   token.AccessIssuer
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewAuthService(
	repo model.Repository,
	tokenStore token.Store,
	tokenIssuer token.AccessIssuer,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
) *AuthService {
	return &AuthService{
		repo:          repo,
		tokenStore:    tokenStore,
		tokenIssuer:   tokenIssuer,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterDTO) (*model.User, error) {
	email, err := model.NewEmail(req.Email)
	if err != nil {
		return nil, apperror.InvalidInput(err.Error())
	}

	exists, err := s.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("email already exists")
	}

	password, err := model.NewPassword(req.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	return s.repo.Create(ctx, model.New(req.Name, email, password))
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginDTO) (TokenPair, error) {
	email, err := model.NewEmail(req.Email)
	if err != nil {
		return TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	if !user.Password.Matches(req.Password) {
		return TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	return s.issueTokenPair(ctx, user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	userID, err := s.tokenStore.UserID(ctx, refreshToken)
	if err != nil {
		return TokenPair{}, err
	}

	if err := s.tokenStore.Delete(ctx, refreshToken); err != nil {
		return TokenPair{}, apperror.Internal(err)
	}

	return s.issueTokenPair(ctx, userID)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.tokenStore.Delete(ctx, refreshToken)
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID int) (TokenPair, error) {
	accessToken, err := s.tokenIssuer.Issue(userID, s.accessExpiry)
	if err != nil {
		return TokenPair{}, apperror.Internal(err)
	}

	refreshToken := uuid.NewString()
	if err := s.tokenStore.Save(ctx, refreshToken, userID, s.refreshExpiry); err != nil {
		return TokenPair{}, apperror.Internal(err)
	}

	return TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  int(s.accessExpiry.Seconds()),
		RefreshExpiresIn: int(s.refreshExpiry.Seconds()),
	}, nil
}
