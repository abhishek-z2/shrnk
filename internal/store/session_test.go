package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStore_CreateAndGetSession(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(
		ctx,
		"postgres://postgres:postgres@localhost:5433/shrnk?sslmode=disable",
	)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	s := NewPostgresStore(pool)

	// Create a user because sessions must reference an existing user.
	email := "session-test@example.com"

	user, err := s.CreateUser(ctx, email, "test-password-hash")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf("failed to clean up test user: %v", err)
		}
	})

	sessionID := uuid.New()
	tokenHash := "test-token-hash"
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Microsecond)

	err = s.CreateSession(
		ctx,
		sessionID,
		user.ID,
		tokenHash,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM sessions WHERE id = $1",
			sessionID,
		)
		if err != nil {
			t.Errorf("failed to clean up test session: %v", err)
		}
	})

	session, err := s.GetSession(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if session.ID != sessionID {
		t.Errorf("expected session ID %v, got %v", sessionID, session.ID)
	}

	if session.UserID != user.ID {
		t.Errorf("expected user ID %d, got %d", user.ID, session.UserID)
	}

	if session.TokenHash != tokenHash {
		t.Errorf(
			"expected token hash %q, got %q",
			tokenHash,
			session.TokenHash,
		)
	}

	if !session.ExpiresAt.Equal(expiresAt) {
		t.Errorf(
			"expected expiration %v, got %v",
			expiresAt,
			session.ExpiresAt,
		)
	}
}

func TestPostgresStore_GetSession_NotFound(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(
		ctx,
		"postgres://postgres:postgres@localhost:5433/shrnk?sslmode=disable",
	)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	s := NewPostgresStore(pool)

	_, err = s.GetSession(ctx, "definitely-does-not-exist")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresStore_DeleteSession(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(
		ctx,
		"postgres://postgres:postgres@localhost:5433/shrnk?sslmode=disable",
	)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	s := NewPostgresStore(pool)

	// Create a user because sessions have a foreign-key relationship to users.
	user, err := s.CreateUser(
		ctx,
		"session-delete-test@example.com",
		"test-password-hash",
	)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf("failed to clean up test user: %v", err)
		}
	})

	sessionID := uuid.New()

	err = s.CreateSession(
		ctx,
		sessionID,
		user.ID,
		"delete-test-token-hash",
		time.Now().Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	err = s.DeleteSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// The session should no longer be retrievable.
	_, err = s.GetSession(ctx, "delete-test-token-hash")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected deleted session to return ErrNotFound, got %v", err)
	}
}
