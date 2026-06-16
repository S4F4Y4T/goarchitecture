package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	userDomain "github.com/s4f4y4t/go-microservice/services/user/internal/domain/user"
	"github.com/s4f4y4t/go-microservice/services/user/internal/usecase/port"
)

type AuthService struct {
	repo          userDomain.Repository
	tokenStore    token.Store
	tokenIssuer   token.AccessIssuer
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

var _ port.AuthUseCase = (*AuthService)(nil)

func NewAuthService(
	repo userDomain.Repository,
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

func (s *AuthService) Register(ctx context.Context, input port.RegisterInput) (*userDomain.User, error) {
	email, err := userDomain.NewEmail(input.Email)
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

	password, err := userDomain.NewPassword(input.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	return s.repo.Create(ctx, userDomain.New(input.Name, email, password))
}

func (s *AuthService) Login(ctx context.Context, input port.LoginInput) (port.TokenPair, error) {
	email, err := userDomain.NewEmail(input.Email)
	if err != nil {
		return port.TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return port.TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	if !user.Password.Matches(input.Password) {
		return port.TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	return s.issueTokenPair(ctx, user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (port.TokenPair, error) {
	userID, err := s.tokenStore.UserID(ctx, refreshToken)
	if err != nil {
		return port.TokenPair{}, err
	}

	if err := s.tokenStore.Delete(ctx, refreshToken); err != nil {
		return port.TokenPair{}, apperror.Internal(err)
	}

	return s.issueTokenPair(ctx, userID)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.tokenStore.Delete(ctx, refreshToken)
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID int) (port.TokenPair, error) {
	accessToken, err := s.tokenIssuer.Issue(userID, s.accessExpiry)
	if err != nil {
		return port.TokenPair{}, apperror.Internal(err)
	}

	refreshToken := uuid.NewString()
	if err := s.tokenStore.Save(ctx, refreshToken, userID, s.refreshExpiry); err != nil {
		return port.TokenPair{}, apperror.Internal(err)
	}

	return port.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  int(s.accessExpiry.Seconds()),
		RefreshExpiresIn: int(s.refreshExpiry.Seconds()),
	}, nil
}
