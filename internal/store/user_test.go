package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStore_CreateAndGetUser(t *testing.T) {
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

	email := "user-store-test@example.com"
	passwordHash := "test-password-hash"

	user, err := s.CreateUser(ctx, email, passwordHash)
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

	if user.Email != email {
		t.Errorf("expected email %q, got %q", email, user.Email)
	}

	if user.PasswordHash != passwordHash {
		t.Errorf(
			"expected password hash %q, got %q",
			passwordHash,
			user.PasswordHash,
		)
	}

	if user.ID == 0 {
		t.Error("expected user ID to be generated")
	}

	found, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}

	if found.ID != user.ID {
		t.Errorf("expected ID %d, got %d", user.ID, found.ID)
	}

	if found.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, found.Email)
	}

	if found.PasswordHash != user.PasswordHash {
		t.Errorf(
			"expected password hash %q, got %q",
			user.PasswordHash,
			found.PasswordHash,
		)
	}
}

func TestPostgresStore_GetUserByEmail_NotFound(t *testing.T) {
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

	_, err = s.GetUserByEmail(
		ctx,
		"definitely-does-not-exist@example.com",
	)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresStore_CreateUserWithSession(t *testing.T) {
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

	// Unique email avoids conflicts when tests are run repeatedly.
	email := uuid.NewString() + "@example.com"
	sessionID := uuid.New()
	tokenHash := uuid.NewString()
	expiresAt := time.Now().Add(time.Hour)

	user, err := s.CreateUserWithSession(
		ctx,
		email,
		"test-password-hash",
		sessionID,
		tokenHash,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("CreateUserWithSession failed: %v", err)
	}

	// Clean up both records. The session must be deleted before its user
	// because sessions.user_id references users.id.
	t.Cleanup(func() {
		if err := s.DeleteSession(ctx, sessionID); err != nil {
			t.Errorf("failed to clean up session: %v", err)
		}

		_, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		if err != nil {
			t.Errorf("failed to clean up user: %v", err)
		}
	})

	// Verify that the session was persisted and linked to the new user.
	session, err := s.GetSession(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if session.UserID != user.ID {
		t.Errorf(
			"expected session user ID %d, got %d",
			user.ID,
			session.UserID,
		)
	}

	if session.ID != sessionID {
		t.Errorf(
			"expected session ID %s, got %s",
			sessionID,
			session.ID,
		)
	}

	if user.Email != email {
		t.Errorf("expected email %q, got %q", email, user.Email)
	}
}
