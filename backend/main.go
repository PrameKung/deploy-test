package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

func main() {
	e := echo.New()
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
	log.Fatal(e.Start(":8080"))
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
            "content": {
              "text/plain": {
                "schema": { "type": "string", "example": "Hello, World!" }
              }
            }
          }
        }
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
