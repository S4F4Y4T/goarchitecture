package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	pkgconfig "github.com/s4f4y4t/go-microservice/pkg/config"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

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

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	PrivateKey    *rsa.PrivateKey
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	CookieSecure  bool
}

type CORSConfig struct {
	AllowedOrigins []string
}

type Config struct {
	Port  int
	DB    DBConfig
	Redis RedisConfig
	JWT   JWTConfig
	CORS  CORSConfig
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	portInt := pkgconfig.GetEnvInt("PORT", 0)
	if portInt == 0 {
		return nil, fmt.Errorf("PORT is missing or invalid")
	}

	db, err := loadDBConfig()
	if err != nil {
		return nil, err
	}

	redis, err := loadRedisConfig()
	if err != nil {
		return nil, err
	}

	jwt, err := loadJWTConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:  portInt,
		DB:    db,
		Redis: redis,
		JWT:   jwt,
		CORS:  loadCORSConfig(),
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

	db.MaxOpenConns = pkgconfig.GetEnvInt("DB_MAX_OPEN_CONNS", 25)
	db.MaxIdleConns = pkgconfig.GetEnvInt("DB_MAX_IDLE_CONNS", 25)
	db.ConnMaxLifetime = pkgconfig.GetEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	db.ConnMaxIdleTime = pkgconfig.GetEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)

	return db, nil
}

func loadRedisConfig() (RedisConfig, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return RedisConfig{}, fmt.Errorf("REDIS_ADDR is required")
	}
	return RedisConfig{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       pkgconfig.GetEnvInt("REDIS_DB", 0),
	}, nil
}

func loadJWTConfig() (JWTConfig, error) {
	privPath := os.Getenv("JWT_PRIVATE_KEY_PATH")
	if privPath == "" {
		return JWTConfig{}, fmt.Errorf("JWT_PRIVATE_KEY_PATH is required")
	}

	privateKey, err := loadPrivateKey(privPath)
	if err != nil {
		return JWTConfig{}, fmt.Errorf("loading JWT private key: %w", err)
	}

	return JWTConfig{
		PrivateKey:    privateKey,
		AccessExpiry:  pkgconfig.GetEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		RefreshExpiry: pkgconfig.GetEnvDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		CookieSecure:  pkgconfig.GetEnvBool("COOKIE_SECURE", true),
	}, nil
}

func loadCORSConfig() CORSConfig {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		slog.Warn("config: CORS_ALLOWED_ORIGINS not set, allowing all origins")
		return CORSConfig{AllowedOrigins: []string{"*"}}
	}

	origins := []string{}
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return CORSConfig{AllowedOrigins: origins}
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", path)
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
