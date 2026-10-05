package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"golang.org/x/oauth2"
)

const (
	googleIssuer  = "https://accounts.google.com"
	sessionCookie = "deploy_test_session"
	stateCookie   = "google_oauth_state"
	nonceCookie   = "google_oauth_nonce"
	oauthMaxAge   = 10 * time.Minute
	sessionAge    = 7 * 24 * time.Hour
)

type googleAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	FrontendURL  string
	SessionKey   string
	SecureCookie bool
}

type googleAuth struct {
	db         *pgxpool.Pool
	config     googleAuthConfig
	oauth      *oauth2.Config
	verifier   *oidc.IDTokenVerifier
	sessionKey []byte
}

type googleClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type userProfile struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"emailVerified"`
	Name          string    `json:"name"`
	PictureURL    string    `json:"pictureUrl"`
}

type sessionClaims struct {
	jwt.RegisteredClaims
}

func newGoogleAuth(db *pgxpool.Pool, config googleAuthConfig) (*googleAuth, error) {
	auth := &googleAuth{db: db, config: config}
	if config.ClientID == "" && config.ClientSecret == "" {
		return auth, nil
	}
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, errors.New("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must both be set")
	}
	if len(config.SessionKey) < 32 {
		return nil, errors.New("SESSION_SECRET must contain at least 32 bytes when Google login is enabled")
	}
	if config.RedirectURL == "" {
		return nil, errors.New("GOOGLE_REDIRECT_URL must be set when Google login is enabled")
	}

	provider, err := oidc.NewProvider(context.Background(), googleIssuer)
	if err != nil {
		return nil, err
	}
	auth.oauth = &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	auth.verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	auth.sessionKey = []byte(config.SessionKey)
	return auth, nil
}

func (a *googleAuth) startLogin(c *echo.Context) error {
	if a.oauth == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Google login is not configured")
	}
	state, err := randomValue(32)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not start Google login")
	}
	nonce, err := randomValue(32)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not start Google login")
	}
	setTemporaryCookie(c, stateCookie, state, a.config.SecureCookie)
	setTemporaryCookie(c, nonceCookie, nonce, a.config.SecureCookie)
	return c.Redirect(http.StatusFound, a.oauth.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce)))
}

func (a *googleAuth) finishLogin(c *echo.Context) error {
	if a.oauth == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Google login is not configured")
	}
	stateCookieValue, stateErr := c.Cookie(stateCookie)
	nonceCookieValue, nonceErr := c.Cookie(nonceCookie)
	clearCookie(c, stateCookie, a.config.SecureCookie)
	clearCookie(c, nonceCookie, a.config.SecureCookie)
	state := c.QueryParam("state")
	if stateErr != nil || nonceErr != nil || state == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(stateCookieValue.Value)) != 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid OAuth state")
	}
	if providerError := c.QueryParam("error"); providerError != "" {
		return c.Redirect(http.StatusFound, a.config.FrontendURL+"/?login=cancelled")
	}
	code := c.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing OAuth code")
	}
	token, err := a.oauth.Exchange(c.Request().Context(), code)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Google token exchange failed")
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return echo.NewHTTPError(http.StatusBadGateway, "Google did not return an identity token")
	}
	idToken, err := a.verifier.Verify(c.Request().Context(), rawIDToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Google identity verification failed")
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonceCookieValue.Value)) != 1 {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid OAuth nonce")
	}
	var claims googleClaims
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Email == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "Google profile is incomplete")
	}

	profile, err := a.saveUser(c.Request().Context(), claims)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not save user profile")
	}
	session, err := a.createSession(profile.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not create login session")
	}
	http.SetCookie(c.Response(), &http.Cookie{
		Name: sessionCookie, Value: session, Path: "/", HttpOnly: true,
		Secure: a.config.SecureCookie, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionAge.Seconds()), Expires: time.Now().Add(sessionAge),
	})
	return c.Redirect(http.StatusFound, a.config.FrontendURL+"/?login=success")
}

func (a *googleAuth) currentUser(c *echo.Context) error {
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	if a.oauth == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Google login is not configured")
	}
	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not signed in")
	}
	claims := new(sessionClaims)
	_, err = jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected session signing method")
		}
		return a.sessionKey, nil
	}, jwt.WithIssuer("deploy-test"), jwt.WithExpirationRequired())
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not signed in")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not signed in")
	}
	profile, err := a.findUser(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusUnauthorized, "not signed in")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load user profile")
	}
	return c.JSON(http.StatusOK, profile)
}

func (a *googleAuth) logout(c *echo.Context) error {
	clearCookie(c, sessionCookie, a.config.SecureCookie)
	return c.NoContent(http.StatusNoContent)
}

func (a *googleAuth) saveUser(ctx context.Context, claims googleClaims) (userProfile, error) {
	profile := userProfile{ID: uuid.New(), Email: claims.Email, EmailVerified: claims.EmailVerified, Name: claims.Name, PictureURL: claims.Picture}
	err := a.db.QueryRow(ctx, `
		INSERT INTO users (id, google_sub, email, email_verified, name, picture_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			email_verified = EXCLUDED.email_verified,
			name = EXCLUDED.name,
			picture_url = EXCLUDED.picture_url,
			updated_at = NOW()
		RETURNING id, email, email_verified, name, picture_url
	`, profile.ID, claims.Subject, profile.Email, profile.EmailVerified, profile.Name, profile.PictureURL).Scan(
		&profile.ID, &profile.Email, &profile.EmailVerified, &profile.Name, &profile.PictureURL,
	)
	return profile, err
}

func (a *googleAuth) findUser(ctx context.Context, id uuid.UUID) (userProfile, error) {
	var profile userProfile
	err := a.db.QueryRow(ctx, `
		SELECT id, email, email_verified, name, picture_url
		FROM users WHERE id = $1
	`, id).Scan(&profile.ID, &profile.Email, &profile.EmailVerified, &profile.Name, &profile.PictureURL)
	return profile, err
}

func (a *googleAuth) createSession(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := sessionClaims{RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "deploy-test", Subject: userID.String(),
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(sessionAge)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.sessionKey)
}

func randomValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func setTemporaryCookie(c *echo.Context, name, value string, secure bool) {
	http.SetCookie(c.Response(), &http.Cookie{
		Name: name, Value: value, Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(oauthMaxAge.Seconds()),
		Expires: time.Now().Add(oauthMaxAge),
	})
}

func clearCookie(c *echo.Context, name string, secure bool) {
	http.SetCookie(c.Response(), &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0),
	})
}
