# deploy-test

## Backend Google login

The backend stores Google user profiles in PostgreSQL. On first login it creates a row keyed by Google's stable account subject; later logins update the saved email, verification status, name, and profile picture.

Copy `.env.sample` to `.env`, then set the Google OAuth credentials and generate a session secret:

```sh
cp .env.sample .env
# Edit .env with your Google OAuth client ID, client secret, and a random session secret.
docker compose up --build
```

In Google Cloud, create an OAuth 2.0 Web application client and add `http://localhost:8080/auth/google/callback` as an authorized redirect URI. Open `http://localhost:8080/auth/google` to start login. After login the backend redirects to the frontend; the frontend can request `GET http://localhost:8080/api/me` with credentials to retrieve the saved profile. `POST http://localhost:8080/auth/logout` clears the session.

The default local PostgreSQL service stores its data in the `postgres-data` Compose volume. For deployment, set `DATABASE_URL`, `FRONTEND_URL`, `GOOGLE_REDIRECT_URL`, and a strong `SESSION_SECRET`; use HTTPS so the session cookie is marked secure. `GET /api/me` returns `id`, `email`, `emailVerified`, `name`, and `pictureUrl`.
