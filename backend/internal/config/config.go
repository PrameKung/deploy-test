package config

import (
	"os"
	"strings"
)

type Config struct {
	Address        string
	DatabaseURL    string
	FrontendURL    string
	GoogleClientID string
	GoogleSecret   string
	GoogleRedirect string
	SessionSecret  string
	SecureCookies  bool
}

func Load() Config {
	redirectURL := envOr("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback")
	secureCookies := strings.HasPrefix(redirectURL, "https://")
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		secureCookies = strings.EqualFold(value, "true")
	}

	return Config{
		Address:        envOr("BACKEND_ADDRESS", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		FrontendURL:    strings.TrimRight(envOr("FRONTEND_URL", "http://localhost:3000"), "/"),
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirect: redirectURL,
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		SecureCookies:  secureCookies,
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
