package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()
	c := Config{
		Port: env("HTTP_PORT", "8080"), DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret: os.Getenv("JWT_SECRET"), AccessTTL: 5 * time.Minute, RefreshTTL: 7 * 24 * time.Hour,
	}
	if c.DatabaseURL == "" || c.JWTSecret == "" {
		return c, fmt.Errorf("DATABASE_URL and JWT_SECRET are required")
	}
	return c, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
