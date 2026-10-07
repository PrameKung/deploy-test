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

In Google Cloud, create an OAuth 2.0 Web application client and add the backend callback URL as an authorized redirect URI. Locally, this is `http://localhost:8080/auth/google/callback`; in production, use `https://<render-service>.onrender.com/auth/google/callback`. Open `/auth/google` on the backend to start login. After login the backend redirects to the frontend; the frontend can request `/api/me` with credentials to retrieve the saved profile. `POST /auth/logout` clears the session.

Backend settings live in `backend/.env`; frontend settings live in `frontend/.env`. `DATABASE_URL` points the backend to Neon. For production, set the Vercel environment variable `BACKEND_PUBLIC_URL` to `https://<render-service>.onrender.com`. On Render, set `FRONTEND_URL` to `https://<vercel-project>.vercel.app`, `GOOGLE_REDIRECT_URL` to `https://<render-service>.onrender.com/auth/google/callback`, and `COOKIE_SECURE=true`. Configure the same callback URL in Google Cloud. `GET /api/me` returns `id`, `email`, `emailVerified`, `name`, and `pictureUrl`.
