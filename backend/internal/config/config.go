package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv       string
	HTTPAddr     string
	DatabaseURL  string
	CookieSecret string
}

func Load() (Config, error) {
	// Best-effort: try the repo-root .env when running natively (`go run ./cmd/api`
	// from the backend/ directory). Ignored if absent (e.g. in container).
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := Config{
		AppEnv:       getenv("APP_ENV", "dev"),
		HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		CookieSecret: os.Getenv("COOKIE_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.CookieSecret == "" {
		return cfg, fmt.Errorf("COOKIE_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
