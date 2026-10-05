package httpapi

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"backend/internal/auth"
)

func New(frontendURL string, googleAuth *auth.Google) *echo.Echo {
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
	googleAuth.RegisterRoutes(e)
	return e
}
