package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port              string
	DatabaseURL       string
	SessionSecret     string
	CORSOrigin        string
	RedisURL          string
	LockTTL           time.Duration
	LockRenewInterval time.Duration
}

// Load reads configuration from environment variables.
// It returns an error if required variables are missing.
func Load() (*Config, error) {
	lockTTL, err := parseDuration("LOCK_TTL", "30s")
	if err != nil {
		return nil, err
	}
	lockRenew, err := parseDuration("LOCK_RENEW_INTERVAL", "10s")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SessionSecret:     os.Getenv("SESSION_SECRET"),
		CORSOrigin:        getEnv("CORS_ORIGIN", "http://localhost:5173"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379"),
		LockTTL:           lockTTL,
		LockRenewInterval: lockRenew,
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func parseDuration(key, defaultValue string) (time.Duration, error) {
	raw := getEnv(key, defaultValue)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return d, nil
}
