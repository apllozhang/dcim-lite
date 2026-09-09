package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	JWTExpiresIn   time.Duration
	AdminUser      string
	AdminPass      string
	AdminDisplayName string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AdminUser:      getEnv("ADMIN_USERNAME", "admin"),
		AdminPass:      os.Getenv("ADMIN_PASSWORD"),
		AdminDisplayName: getEnv("ADMIN_DISPLAY_NAME", "系统管理员"),
	}

	ttl := 7200
	if v := os.Getenv("JWT_TTL_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid JWT_TTL_SECONDS: %q", v)
		}
		ttl = n
	}
	cfg.JWTExpiresIn = time.Duration(ttl) * time.Second

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
