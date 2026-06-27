package config

import (
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

type RabbitMQConfig struct {
	URL      string
	Exchange string
}

type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromName    string
	FromAddress string
	UseTLS      bool
}

type ConsumerConfig struct {
	QueueName  string
	RoutingKey string
	MaxRetries int
	RetryTTL   time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

type Config struct {
	Port     int
	DB       DBConfig
	RabbitMQ RabbitMQConfig
	SMTP     SMTPConfig
	Consumer ConsumerConfig
	CORS     CORSConfig
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

	rabbitmq, err := loadRabbitMQConfig()
	if err != nil {
		return nil, err
	}

	smtp, err := loadSMTPConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:     portInt,
		DB:       db,
		RabbitMQ: rabbitmq,
		SMTP:     smtp,
		Consumer: loadConsumerConfig(),
		CORS:     loadCORSConfig(),
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

func loadRabbitMQConfig() (RabbitMQConfig, error) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		return RabbitMQConfig{}, fmt.Errorf("RABBITMQ_URL is required")
	}
	exchange := os.Getenv("RABBITMQ_EXCHANGE")
	if exchange == "" {
		exchange = "domain_events"
	}
	return RabbitMQConfig{URL: url, Exchange: exchange}, nil
}

func loadSMTPConfig() (SMTPConfig, error) {
	host := os.Getenv("NOTIFICATION_SMTP_HOST")
	if host == "" {
		return SMTPConfig{}, fmt.Errorf("NOTIFICATION_SMTP_HOST is required")
	}
	fromAddress := os.Getenv("NOTIFICATION_SMTP_FROM_ADDRESS")
	if fromAddress == "" {
		return SMTPConfig{}, fmt.Errorf("NOTIFICATION_SMTP_FROM_ADDRESS is required")
	}

	return SMTPConfig{
		Host:        host,
		Port:        pkgconfig.GetEnvInt("NOTIFICATION_SMTP_PORT", 587),
		Username:    os.Getenv("NOTIFICATION_SMTP_USERNAME"),
		Password:    os.Getenv("NOTIFICATION_SMTP_PASSWORD"),
		FromName:    os.Getenv("NOTIFICATION_SMTP_FROM_NAME"),
		FromAddress: fromAddress,
		UseTLS:      pkgconfig.GetEnvBool("NOTIFICATION_SMTP_USE_TLS", true),
	}, nil
}

func loadConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		QueueName:  "notification.user_created",
		RoutingKey: "user.created",
		MaxRetries: pkgconfig.GetEnvInt("NOTIFICATION_CONSUMER_MAX_RETRIES", 3),
		RetryTTL:   pkgconfig.GetEnvDuration("NOTIFICATION_CONSUMER_RETRY_TTL", 30*time.Second),
	}
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
