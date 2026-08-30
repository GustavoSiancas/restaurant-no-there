package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	AllowedOrigins []string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	c := Config{
		Port: env("HTTP_PORT", "8080"), DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret: os.Getenv("JWT_SECRET"), AccessTTL: 5 * time.Minute, RefreshTTL: 7 * 24 * time.Hour,
		AllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
	}
	if c.DatabaseURL == "" || c.JWTSecret == "" {
		return c, fmt.Errorf("DATABASE_URL and JWT_SECRET are required")
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
