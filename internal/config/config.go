package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	AccessTTL       time.Duration
	WorkerAccessTTL time.Duration
	RefreshTTL      time.Duration
	AllowedOrigins  []string
	FixedTime       *time.Time
}

func Load() (Config, error) {
	_ = godotenv.Load()
	c := Config{
		Port: env("HTTP_PORT", "8080"), DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
	}
	if c.DatabaseURL == "" || c.JWTSecret == "" {
		return c, fmt.Errorf("DATABASE_URL and JWT_SECRET are required")
	}
	var err error
	if c.AccessTTL, err = time.ParseDuration(env("ACCESS_TOKEN_TTL", "5m")); err != nil || c.AccessTTL <= 0 {
		return c, fmt.Errorf("ACCESS_TOKEN_TTL must be a positive Go duration, for example 5m")
	}
	if c.WorkerAccessTTL, err = time.ParseDuration(env("WORKER_ACCESS_TOKEN_TTL", "45s")); err != nil || c.WorkerAccessTTL <= 0 {
		return c, fmt.Errorf("WORKER_ACCESS_TOKEN_TTL must be a positive Go duration, for example 45s")
	}
	if c.RefreshTTL, err = time.ParseDuration(env("REFRESH_TOKEN_TTL", "168h")); err != nil || c.RefreshTTL <= 0 {
		return c, fmt.Errorf("REFRESH_TOKEN_TTL must be a positive Go duration, for example 168h")
	}
	if value := strings.TrimSpace(os.Getenv("APP_FIXED_TIME")); value != "" {
		fixedTime, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return c, fmt.Errorf("APP_FIXED_TIME must use RFC3339, for example 2026-08-31T08:00:00-05:00")
		}
		c.FixedTime = &fixedTime
	}
	return c, nil
}

func splitCSV(value string) []string {
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		if origin := strings.TrimSpace(item); origin != "" {
			result = append(result, origin)
		}
	}
	return result
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
