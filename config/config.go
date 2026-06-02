package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string

	// Connection pool tuning. See database.go for how these are applied to the
	// underlying *sql.DB.
	MaxOpenConns    int           // hard cap on total open connections
	MaxIdleConns    int           // connections kept ready in the idle pool
	ConnMaxLifetime time.Duration // recycle a connection after this age
	ConnMaxIdleTime time.Duration // close a connection idle for this long
}

type Config struct {
	Port int
	DB   DBConfig
}

func LoadConfig() (*Config, error) {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		return nil, fmt.Errorf("PORT is missing")
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %v", err)
	}

	db, err := loadDBConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port: portInt,
		DB:   db,
	}, nil
}

func loadDBConfig() (DBConfig, error) {
	db := DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}

	missing := []string{}
	if db.Host == "" {
		missing = append(missing, "DB_HOST")
	}
	if db.Port == "" {
		missing = append(missing, "DB_PORT")
	}
	if db.User == "" {
		missing = append(missing, "DB_USER")
	}
	if db.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if db.Name == "" {
		missing = append(missing, "DB_NAME")
	}
	if len(missing) > 0 {
		return DBConfig{}, fmt.Errorf("missing database env vars: %v", missing)
	}

	if db.SSLMode == "" {
		db.SSLMode = "disable"
	}

	// Pool settings are optional; fall back to production-sane defaults.
	db.MaxOpenConns = getEnvInt("DB_MAX_OPEN_CONNS", 25)
	db.MaxIdleConns = getEnvInt("DB_MAX_IDLE_CONNS", 25)
	db.ConnMaxLifetime = getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	db.ConnMaxIdleTime = getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)

	return db, nil
}

// getEnvInt reads an integer env var, returning def when unset or unparseable.
func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("config: invalid %s=%q, using default %d", key, v, def)
		return def
	}
	return n
}

// getEnvDuration reads a Go duration env var (e.g. "5m", "30s"), returning def
// when unset or unparseable.
func getEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("config: invalid %s=%q, using default %s", key, v, def)
		return def
	}
	return d
}
