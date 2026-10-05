package main

import (
	"context"
	"fmt"
	"log"

	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/httpapi"
	"backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()
	ctx := context.Background()
	database, err := store.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer database.Close()

	googleAuth, err := auth.NewGoogle(ctx, database, auth.Config{
		ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleSecret,
		RedirectURL: cfg.GoogleRedirect, FrontendURL: cfg.FrontendURL,
		SessionKey: cfg.SessionSecret, SecureCookie: cfg.SecureCookies,
	})
	if err != nil {
		return fmt.Errorf("configure Google login: %w", err)
	}

	server := httpapi.New(cfg.FrontendURL, googleAuth)
	return server.Start(cfg.Address)
}
