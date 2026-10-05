package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID            uuid.UUID
	GoogleSubject string
	Email         string
	EmailVerified bool
	Name          string
	PictureURL    string
}

type Postgres struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) SaveGoogleUser(ctx context.Context, subject, email string, emailVerified bool, name, pictureURL string) (User, error) {
	user := User{
		ID: uuid.New(), GoogleSubject: subject, Email: email,
		EmailVerified: emailVerified, Name: name, PictureURL: pictureURL,
	}
	err := p.pool.QueryRow(ctx, `
		INSERT INTO users (id, google_sub, email, email_verified, name, picture_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			email_verified = EXCLUDED.email_verified,
			name = EXCLUDED.name,
			picture_url = EXCLUDED.picture_url,
			updated_at = NOW()
		RETURNING id, google_sub, email, email_verified, name, picture_url
	`, user.ID, subject, email, emailVerified, name, pictureURL).Scan(
		&user.ID, &user.GoogleSubject, &user.Email, &user.EmailVerified, &user.Name, &user.PictureURL,
	)
	return user, err
}

func (p *Postgres) FindUser(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := p.pool.QueryRow(ctx, `
		SELECT id, google_sub, email, email_verified, name, picture_url
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.GoogleSubject, &user.Email, &user.EmailVerified, &user.Name, &user.PictureURL)
	return user, err
}

const schema = `
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
`
