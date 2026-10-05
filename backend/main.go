package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	databaseURL := envOr("DATABASE_URL", "postgres://app:app@localhost:5432/deploytest?sslmode=disable")
	ctx := context.Background()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("configure database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		log.Fatalf("initialize database: %v", err)
	}

	frontendURL := strings.TrimRight(envOr("FRONTEND_URL", "http://localhost:3000"), "/")
	secureCookies := strings.HasPrefix(envOr("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"), "https://")
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		secureCookies = strings.EqualFold(value, "true")
	}
	auth, err := newGoogleAuth(db, googleAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  envOr("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
		FrontendURL:  frontendURL,
		SessionKey:   os.Getenv("SESSION_SECRET"),
		SecureCookie: secureCookies,
	})
	if err != nil {
		log.Fatalf("configure Google login: %v", err)
	}

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{frontendURL},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderContentType},
		AllowCredentials: true,
	}))
	e.GET("/", func(c *echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.GET("/openapi.json", func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSONCharsetUTF8)
		return c.String(http.StatusOK, openAPISpec)
	})
	e.GET("/docs", func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return c.String(http.StatusOK, strings.ReplaceAll(scalarPage, "{{OPENAPI_URL}}", "/openapi.json"))
	})
	e.GET("/auth/google", auth.startLogin)
	e.GET("/auth/google/callback", auth.finishLogin)
	e.GET("/api/me", auth.currentUser)
	e.POST("/auth/logout", auth.logout)
	log.Fatal(e.Start(":8080"))
}

func migrate(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			google_sub TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL,
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			name TEXT NOT NULL DEFAULT '',
			picture_url TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS users_email_idx ON users (email);
	`)
	return err
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Deploy Test API",
    "version": "1.0.0",
    "description": "HTTP API for the deploy-test service."
  },
  "servers": [{ "url": "/" }],
  "paths": {
    "/": {
      "get": {
        "summary": "Health check",
        "description": "Returns a basic response to confirm the service is reachable.",
        "operationId": "getRoot",
        "responses": {
          "200": {
            "description": "Service is reachable.",
            "content": { "text/plain": { "schema": { "type": "string", "example": "Hello, World!" } } }
          }
        }
      }
    },
    "/auth/google": {
      "get": {
        "summary": "Start Google login",
        "responses": { "302": { "description": "Redirects to Google sign-in." } }
      }
    },
    "/auth/google/callback": {
      "get": {
        "summary": "Complete Google login",
        "responses": { "302": { "description": "Saves the user and redirects to the frontend." } }
      }
    },
    "/api/me": {
      "get": {
        "summary": "Get the signed-in user",
        "responses": {
          "200": { "description": "Signed-in user profile." },
          "401": { "description": "No valid login session." }
        }
      }
    },
    "/auth/logout": {
      "post": {
        "summary": "End the login session",
        "responses": { "204": { "description": "Session ended." } }
      }
    }
  }
}`

const scalarPage = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Deploy Test API Reference</title>
  </head>
  <body>
    <script id="api-reference" data-url="{{OPENAPI_URL}}"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
