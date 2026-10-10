package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStore_CreateAndGetURL(t *testing.T) {
	ctx := context.Background()

	// connect to the database
	pool, err := pgxpool.New(ctx, "postgres://postgres:postgres@localhost:5433/shrnk?sslmode=disable")
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

	longURL := "https://example.com/integration-test"
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Microsecond)

	_, shortCode, err := s.CreateURL(ctx, longURL, expiresAt)
	if err != nil {
		t.Fatalf("CreateURL failed: %v", err)
	}

	// clean up the row even if a later assertion fails.
	t.Cleanup(func() {
		_, err := pool.Exec(ctx,
			"DELETE FROM urls WHERE short_code = $1", shortCode)
		if err != nil {
			t.Errorf("failed to clean up test URL: %v", err)
		}
	})

	record, err := s.GetURL(ctx, shortCode)
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}

	if record.ShortCode != shortCode {
		t.Errorf("expected short code %q, got %q",
			shortCode, record.ShortCode)
	}

	if record.LongURL != longURL {
		t.Errorf("expected long URL %q, got %q",
			longURL, record.LongURL)
	}

	if !record.ExpiresAt.Equal(expiresAt) {
		t.Errorf("expected expiration %v, got %v",
			expiresAt, record.ExpiresAt)
	}
}

func TestPostgresStore_GetURL_NotFound(t *testing.T) {
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

	_, err = s.GetURL(ctx, "definitely-does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresStore_SweepExpired(t *testing.T) {
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

	// Create one expired URL and one still-valid URL.
	_, expiredCode, err := s.CreateURL(
		ctx,
		"https://example.com/expired-test",
		time.Now().Add(-time.Hour).Truncate(time.Microsecond),
	)
	if err != nil {
		t.Fatalf("failed to create expired URL: %v", err)
	}

	_, validCode, err := s.CreateURL(
		ctx,
		"https://example.com/valid-test",
		time.Now().Add(time.Hour).Truncate(time.Microsecond),
	)
	if err != nil {
		t.Fatalf("failed to create valid URL: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM urls WHERE short_code IN ($1, $2)",
			expiredCode,
			validCode,
		)
		if err != nil {
			t.Errorf("failed to clean up test URLs: %v", err)
		}
	})

	if err := s.SweepExpired(ctx); err != nil {
		t.Fatalf("SweepExpired failed: %v", err)
	}

	_, err = s.GetURL(ctx, expiredCode)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected expired URL to be deleted, got error: %v", err)
	}

	_, err = s.GetURL(ctx, validCode)
	if err != nil {
		t.Errorf("expected valid URL to remain, got error: %v", err)
	}
}
