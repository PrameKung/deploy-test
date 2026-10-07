package auth

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
	"github.com/labstack/echo/v5"
	"golang.org/x/oauth2"

	"backend/internal/store"
)

const (
	googleIssuer  = "https://accounts.google.com"
	sessionCookie = "deploy_test_session"
	stateCookie   = "google_oauth_state"
	nonceCookie   = "google_oauth_nonce"
	oauthMaxAge   = 10 * time.Minute
	sessionAge    = 7 * 24 * time.Hour
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	FrontendURL  string
	SessionKey   string
	SecureCookie bool
}

type UserRepository interface {
	SaveGoogleUser(context.Context, string, string, bool, string, string) (store.User, error)
	FindUser(context.Context, uuid.UUID) (store.User, error)
}

type Google struct {
	users      UserRepository
	config     Config
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

func NewGoogle(ctx context.Context, users UserRepository, config Config) (*Google, error) {
	auth := &Google{users: users, config: config}
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

	provider, err := oidc.NewProvider(ctx, googleIssuer)
	if err != nil {
		return nil, err
	}
	auth.oauth = &oauth2.Config{
		ClientID: config.ClientID, ClientSecret: config.ClientSecret,
		RedirectURL: config.RedirectURL, Endpoint: provider.Endpoint(),
		Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
	}
	auth.verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	auth.sessionKey = []byte(config.SessionKey)
	return auth, nil
}

func (a *Google) RegisterRoutes(e *echo.Echo) {
	e.GET("/auth/google", a.startLogin)
	e.GET("/auth/google/callback", a.finishLogin)
	e.GET("/api/me", a.currentUser)
	e.POST("/auth/logout", a.logout)
}

func (a *Google) startLogin(c *echo.Context) error {
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

func (a *Google) finishLogin(c *echo.Context) error {
	if a.oauth == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Google login is not configured")
	}
	stateValue, stateErr := c.Cookie(stateCookie)
	nonceValue, nonceErr := c.Cookie(nonceCookie)
	clearCookie(c, stateCookie, a.config.SecureCookie)
	clearCookie(c, nonceCookie, a.config.SecureCookie)
	state := c.QueryParam("state")
	if stateErr != nil || nonceErr != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(stateValue.Value)) != 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid OAuth state")
	}
	if c.QueryParam("error") != "" {
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
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonceValue.Value)) != 1 {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid OAuth nonce")
	}
	var claims googleClaims
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Email == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "Google profile is incomplete")
	}
	user, err := a.users.SaveGoogleUser(c.Request().Context(), claims.Subject, claims.Email, claims.EmailVerified, claims.Name, claims.Picture)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not save user profile")
	}
	session, err := a.createSession(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not create login session")
	}
	http.SetCookie(c.Response(), &http.Cookie{
		Name: sessionCookie, Value: session, Path: "/", HttpOnly: true,
		Secure: a.config.SecureCookie, SameSite: cookieSameSite(a.config.SecureCookie),
		MaxAge: int(sessionAge.Seconds()), Expires: time.Now().Add(sessionAge),
	})
	return c.Redirect(http.StatusFound, a.config.FrontendURL+"/?login=success")
}

func (a *Google) currentUser(c *echo.Context) error {
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
	user, err := a.users.FindUser(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusUnauthorized, "not signed in")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load user profile")
	}
	return c.JSON(http.StatusOK, userProfile{
		ID: user.ID, Email: user.Email, EmailVerified: user.EmailVerified,
		Name: user.Name, PictureURL: user.PictureURL,
	})
}

func (a *Google) logout(c *echo.Context) error {
	clearCookie(c, sessionCookie, a.config.SecureCookie)
	return c.NoContent(http.StatusNoContent)
}

func (a *Google) createSession(userID uuid.UUID) (string, error) {
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
		SameSite: cookieSameSite(secure), MaxAge: int(oauthMaxAge.Seconds()),
		Expires: time.Now().Add(oauthMaxAge),
	})
}

func clearCookie(c *echo.Context, name string, secure bool) {
	http.SetCookie(c.Response(), &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true, Secure: secure,
		SameSite: cookieSameSite(secure), MaxAge: -1, Expires: time.Unix(0, 0),
	})
}

func cookieSameSite(secure bool) http.SameSite {
	if secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}
