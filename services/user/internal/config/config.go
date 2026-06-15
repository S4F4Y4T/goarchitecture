package config

import (
	"fmt"
	"log/slog"
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

	// Connection pool tuning. See database.go for how these are applied to the
	// underlying *sql.DB.
	MaxOpenConns    int           // hard cap on total open connections
	MaxIdleConns    int           // connections kept ready in the idle pool
	ConnMaxLifetime time.Duration // recycle a connection after this age
	ConnMaxIdleTime time.Duration // close a connection idle for this long
}

type CORSConfig struct {
	// AllowedOrigins lists the origins allowed to make cross-origin requests.
	// A single "*" entry allows any origin.
	AllowedOrigins []string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RateLimitConfig struct {
	Requests int
	Window   time.Duration
}

type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	CookieSecure  bool // set false in local HTTP dev; true in production (HTTPS required)
}

type Config struct {
	Port      int
	DB        DBConfig
	CORS      CORSConfig
	Redis     *RedisConfig // nil when REDIS_ADDR is unset; rate limiting is disabled
	RateLimit RateLimitConfig
	JWT       JWTConfig
}

func LoadConfig() (*Config, error) {

	// best-effort — in Docker env vars come from the container environment
	_ = godotenv.Load()

	portInt := pkgconfig.GetEnvInt("PORT", 0)
	if portInt == 0 {
		return nil, fmt.Errorf("PORT is missing or invalid")
	}

	db, err := loadDBConfig()
	if err != nil {
		return nil, err
	}

	jwt, err := loadJWTConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:      portInt,
		DB:        db,
		CORS:      loadCORSConfig(),
		Redis:     loadRedisConfig(),
		RateLimit: loadRateLimitConfig(),
		JWT:       jwt,
	}, nil
}

// loadCORSConfig reads CORS_ALLOWED_ORIGINS as a comma-separated list of
// origins (e.g. "https://app.example.com,https://admin.example.com"),
// defaulting to "*" when unset.
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

func loadRedisConfig() *RedisConfig {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		slog.Warn("config: REDIS_ADDR not set, rate limiting disabled")
		return nil
	}
	return &RedisConfig{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       pkgconfig.GetEnvInt("REDIS_DB", 0),
	}
}

func loadRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Requests: pkgconfig.GetEnvInt("RATE_LIMIT_REQUESTS", 100),
		Window:   pkgconfig.GetEnvDuration("RATE_LIMIT_WINDOW", time.Minute),
	}
}

func loadJWTConfig() (JWTConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return JWTConfig{}, fmt.Errorf("JWT_SECRET is required")
	}
	return JWTConfig{
		Secret:        secret,
		AccessExpiry:  pkgconfig.GetEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		RefreshExpiry: pkgconfig.GetEnvDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		CookieSecure:  pkgconfig.GetEnvBool("COOKIE_SECURE", true),
	}, nil
}
