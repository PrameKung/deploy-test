# deploy-test

## Backend Google login

The backend stores Google user profiles in PostgreSQL. On first login it creates a row keyed by Google's stable account subject; later logins update the saved email, verification status, name, and profile picture.

Copy the separate environment samples, then set the Google OAuth credentials and generate a session secret:

```sh
cp backend/.env.sample backend/.env
cp frontend/.env.sample frontend/.env
openssl rand -hex 32
# Put the generated value in backend/.env as SESSION_SECRET.
# Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET in backend/.env.
docker compose up --build
```

In Google Cloud, create an OAuth 2.0 Web application client and add `http://localhost:8080/auth/google/callback` as an authorized redirect URI. Open `http://localhost:8080/auth/google` to start login. After login the backend redirects to the frontend; the frontend can request `GET http://localhost:8080/api/me` with credentials to retrieve the saved profile. `POST http://localhost:8080/auth/logout` clears the session.

Backend settings live in `backend/.env`; frontend settings live in `frontend/.env`. `BACKEND_PUBLIC_URL` tells the browser which backend URL to call. The default local PostgreSQL service stores its data in the `postgres-data` Compose volume. For deployment, set the backend URLs and a strong `SESSION_SECRET`, use HTTPS so the session cookie is marked secure, and set `BACKEND_PUBLIC_URL` to the browser-accessible API URL. `GET /api/me` returns `id`, `email`, `emailVerified`, `name`, and `pictureUrl`.
