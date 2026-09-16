package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv         string
	Port           string
	DatabaseURL    string
	MigrationsPath string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

func Load() Config {
	return Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		Port:           getEnv("BACKEND_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://symphonia:symphonia@localhost:5432/symphonia?sslmode=disable"),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "migrations"),
		ReadTimeout:    getDurationEnv("HTTP_READ_TIMEOUT_SECONDS", 10*time.Second),
		WriteTimeout:   getDurationEnv("HTTP_WRITE_TIMEOUT_SECONDS", 10*time.Second),
	}
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}
