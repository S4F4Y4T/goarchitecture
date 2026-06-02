package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
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

	return db, nil
}
