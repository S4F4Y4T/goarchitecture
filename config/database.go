package config

import (
	"fmt"
	"net/url"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DSN builds a Postgres connection URL from the individual credentials. The
// user and password are URL-encoded so special characters are handled safely.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		c.Name,
		c.SSLMode,
	)
}

func SetupDatabase(cfg DBConfig) (*gorm.DB, error) {

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
