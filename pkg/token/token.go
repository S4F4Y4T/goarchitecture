// Package token issues and verifies the JWT access tokens used for API
// authentication. Tokens are signed with HMAC-SHA256 and carry the user's ID
// and role as claims.
package token

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is what a verified token asserts about the caller.
type Claims struct {
	UserID int
	Role   string
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Sign issues a token for the given user, valid for the manager's TTL.
func (m *Manager) Sign(userID int, role string) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  strconv.Itoa(userID),
		"role": role,
		"iat":  now.Unix(),
		"exp":  now.Add(m.ttl).Unix(),
	})
	return t.SignedString(m.secret)
}

// Verify parses and validates a token string (signature + expiry) and returns
// its claims.
func (m *Manager) Verify(tokenString string) (*Claims, error) {
	t, err := jwt.Parse(tokenString,
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		// Pinning the algorithm prevents downgrade tricks like alg=none or an
		// attacker-chosen algorithm.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	mc, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token: unexpected claims type")
	}

	sub, err := mc.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	userID, err := strconv.Atoi(sub)
	if err != nil {
		return nil, fmt.Errorf("invalid token: non-numeric subject")
	}

	role, _ := mc["role"].(string)
	if role == "" {
		return nil, fmt.Errorf("invalid token: missing role")
	}

	return &Claims{UserID: userID, Role: role}, nil
}
