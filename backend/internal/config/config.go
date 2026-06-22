package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	DatabaseURL    string
	CookieSecret   string
	AttachmentsDir string
}

func Load() (Config, error) {
	cfg, err := loadBase()
	if err != nil {
		return cfg, err
	}
	if cfg.CookieSecret == "" {
		return cfg, fmt.Errorf("COOKIE_SECRET is required")
	}
	return cfg, nil
}

// LoadForJobs loads the same Config but only requires DATABASE_URL.
// The jobs binary does not sign cookies, so COOKIE_SECRET may be empty.
func LoadForJobs() (Config, error) {
	return loadBase()
}

func loadBase() (Config, error) {
	// Best-effort: try the repo-root .env when running natively (`go run ./cmd/api`
	// from the backend/ directory). Ignored if absent (e.g. in container).
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := Config{
		AppEnv:         getenv("APP_ENV", "dev"),
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		CookieSecret:   os.Getenv("COOKIE_SECRET"),
		AttachmentsDir: getenv("RP_ATTACHMENTS_DIR", "data/attachments"),
	}

	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
