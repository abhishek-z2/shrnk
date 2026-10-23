package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
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

func (s *PostgresStore) CreateUserWithSession(
	ctx context.Context,
	email string,
	passwordHash string,
	sessionID uuid.UUID,
	tokenHash string,
	expiresAt time.Time,
) (User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}

	defer tx.Rollback(ctx)

	var user User

	err = tx.QueryRow(
		ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email,
		passwordHash,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		return User{}, err
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO sessions (id, user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		sessionID,
		user.ID,
		tokenHash,
		expiresAt,
	)
	if err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *PostgresStore) GetUserByID(
	ctx context.Context,
	userID int64,
) (User, error) {
	var user User

	err := s.db.QueryRow(
		ctx,
		`SELECT id,email,password_hash,created_at
		FROM users
		WHERE id = $1`,
		userID,
	).Scan(
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
