package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/mail"
)

type Email string

func NewEmail(s string) (Email, error) {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", fmt.Errorf("invalid email: %w", err)
	}
	return Email(addr.Address), nil
}

func (e Email) String() string { return string(e) }

func (e Email) Value() (driver.Value, error) { return string(e), nil }

func (e *Email) Scan(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into Email", value)
	}
	*e = Email(s)
	return nil
}

func (e Email) MarshalJSON() ([]byte, error)  { return json.Marshal(string(e)) }
func (e *Email) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*e = Email(s)
	return nil
}
