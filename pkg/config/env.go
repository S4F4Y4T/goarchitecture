package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

func GetEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("config: invalid env value, using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
}

func GetEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("config: invalid env value, using default", "key", key, "value", v, "default", def.String())
		return def
	}
	return d
}

func GetEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("config: invalid env value, using default", "key", key, "value", v, "default", def)
		return def
	}
	return b
}
