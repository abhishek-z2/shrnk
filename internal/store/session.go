package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Session struct {
	ID        uuid.UUID
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s *PostgresStore) CreateSession(
	ctx context.Context,
	id uuid.UUID,
	userID int64,
	tokenHash string,
	expiresAt time.Time,
) error {
	_, err := s.db.Exec(
		ctx,
		`INSERT INTO sessions (
			id,
			user_id,
			token_hash,
			expires_at
		)
		VALUES ($1, $2, $3, $4)`,
		id,
		userID,
		tokenHash,
		expiresAt,
	)
	return err
}

func (s *PostgresStore) GetSession(
	ctx context.Context,
	tokenHash string,
) (Session, error) {
	var session Session

	err := s.db.QueryRow(
		ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions
		WHERE token_hash = $1`,
		tokenHash,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}

	if err != nil {
		return Session{}, err
	}

	return session, nil
}

func (s *PostgresStore) DeleteSession(
	ctx context.Context,
	id uuid.UUID,
) error {
	_, err := s.db.Exec(
		ctx,
		`DELETE FROM sessions
		WHERE id = $1`,
		id,
	)

	return err
}
