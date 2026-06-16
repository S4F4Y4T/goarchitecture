package token

import (
	"crypto/rsa"
	"time"
)

type AccessIssuer interface {
	Issue(userID int, expiry time.Duration) (string, error)
}

type RSAIssuer struct {
	key *rsa.PrivateKey
}

func NewRSAIssuer(key *rsa.PrivateKey) AccessIssuer {
	return &RSAIssuer{key: key}
}

func (i *RSAIssuer) Issue(userID int, expiry time.Duration) (string, error) {
	return Generate(userID, i.key, expiry)
}
