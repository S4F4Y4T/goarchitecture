package token

import (
	"context"
	"time"
)

// Store persists refresh tokens so they can be validated and revoked.
type Store interface {
	Save(ctx context.Context, token string, userID int, expiry time.Duration) error
	UserID(ctx context.Context, token string) (int, error)
	Delete(ctx context.Context, token string) error
}
