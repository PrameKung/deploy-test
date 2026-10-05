package httpapi

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
