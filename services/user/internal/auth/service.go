package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/token"
	"github.com/s4f4y4t/go-microservice/services/user/internal/user"
	"golang.org/x/crypto/bcrypt"
)

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  int
	RefreshExpiresIn int
}

// UserLookup is the slice of user.Repository that auth actually needs.
// Owning this interface here, instead of depending on the full
// user.Repository, keeps auth's coupling to the user module explicit and
// minimal — any user.Repository implementation already satisfies it.
type UserLookup interface {
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	Create(ctx context.Context, u *user.User) (*user.User, error)
}

type AuthService struct {
	repo          UserLookup
	tokenStore    token.Store
	tokenIssuer   token.AccessIssuer
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewAuthService(
	repo UserLookup,
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

func (s *AuthService) Register(ctx context.Context, req RegisterDTO) (*user.User, error) {
	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
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

	return s.repo.Create(ctx, &user.User{Name: req.Name, Email: req.Email, Password: string(hashed)})
}

func (s *AuthService) Login(ctx context.Context, req LoginDTO) (TokenPair, error) {
	u, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		return TokenPair{}, apperror.Unauthorized("invalid email or password")
	}

	return s.issueTokenPair(ctx, u.ID)
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
