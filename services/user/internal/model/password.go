package model

import (
	"database/sql/driver"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Password string

func NewPassword(plain string) (Password, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return Password(hashed), nil
}

func (p Password) Matches(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(p), []byte(plain)) == nil
}

func (p Password) String() string { return string(p) }

func (p Password) Value() (driver.Value, error) { return string(p), nil }

func (p *Password) Scan(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into Password", value)
	}
	*p = Password(s)
	return nil
}

func (p Password) MarshalJSON() ([]byte, error) { return []byte("null"), nil }
