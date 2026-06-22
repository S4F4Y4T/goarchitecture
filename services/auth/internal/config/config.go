package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	pkgconfig "github.com/s4f4y4t/go-microservice/pkg/config"

	"github.com/joho/godotenv"
)

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
	Port            int
	UserGRPCAddr    string
	UserGRPCTimeout time.Duration
	Redis           RedisConfig
	JWT             JWTConfig
	CORS            CORSConfig
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	portInt := pkgconfig.GetEnvInt("PORT", 0)
	if portInt == 0 {
		return nil, fmt.Errorf("PORT is missing or invalid")
	}

	userGRPCAddr := os.Getenv("USER_GRPC_ADDR")
	if userGRPCAddr == "" {
		return nil, fmt.Errorf("USER_GRPC_ADDR is required")
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
		Port:            portInt,
		UserGRPCAddr:    userGRPCAddr,
		UserGRPCTimeout: pkgconfig.GetEnvDuration("USER_GRPC_TIMEOUT", 5*time.Second),
		Redis:           redis,
		JWT:             jwt,
		CORS:            loadCORSConfig(),
	}, nil
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
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key at %s is not an RSA private key", path)
	}
	return rsaKey, nil
}
