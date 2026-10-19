package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

func (s *PostgresStore) CreateUser(
	ctx context.Context,
	email string,
	passwordHash string,
) (User, error) {

	var user User

	err := s.db.QueryRow(
		ctx, `
		INSERT INTO users(email,password_hash)
		VALUES($1,$2)
		RETURNING id,email,password_hash,created_at
		`, email, passwordHash,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *PostgresStore) GetUserByEmail(
	ctx context.Context,
	email string,
) (User, error) {
	var user User

	err := s.db.QueryRow(
		ctx, `
	SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}

	if err != nil {
		return User{}, err
	}

	return user, nil
}
