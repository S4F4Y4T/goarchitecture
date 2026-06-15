package token

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer is the value placed in the JWT `iss` claim. Kong's jwt plugin uses it
// to look up which consumer credential to verify against.
const Issuer = "go-microservice"

type Claims struct {
	UserID int `json:"uid"`
	jwt.RegisteredClaims
}

func Generate(userID int, privateKey *rsa.PrivateKey, expiry time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
}

func ParseUserID(tokenStr string, publicKey *rsa.PublicKey) (int, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return 0, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return 0, jwt.ErrTokenInvalidClaims
	}
	return claims.UserID, nil
}
