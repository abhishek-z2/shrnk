package store

import (
	"context"
	"errors"
	"time"

	//"log"

	"github.com/abhishek-z2/shrnk/internal/shortcode"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		db: db,
	}
}

type URLRecord struct {
	ID        int64
	ShortCode string
	LongURL   string
	ExpiresAt time.Time
}

func (s *PostgresStore) CreateURL(ctx context.Context, longURL string, expiresAt time.Time) (int64, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(ctx)

	var id int64

	err = tx.QueryRow(
		ctx,
		`SELECT nextval('urls_id_seq')`,
	).Scan(&id)
	if err != nil {
		return 0, "", err
	}

	shortcode := shortcode.Encode(uint64(id))

	_, err = tx.Exec(
		ctx,
		`INSERT INTO urls(id,short_code,long_url,expires_at)
		VALUES ($1,$2,$3,$4)`,
		id,
		shortcode,
		longURL,
		expiresAt,
	)
	if err != nil {
		return 0, "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, "", err
	}

	return id, shortcode, nil
}

var ErrNotFound = errors.New("url not found")

func (s *PostgresStore) GetURL(ctx context.Context, shortCode string) (URLRecord, error) {
	var record URLRecord

	err := s.db.QueryRow(
		ctx,
		`SELECT id,short_code,long_url,expires_at
		FROM urls 
		WHERE short_code=($1)`,
		shortCode,
	).Scan(
		&record.ID,
		&record.ShortCode,
		&record.LongURL,
		&record.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return URLRecord{}, ErrNotFound
	}
	if err != nil {
		return URLRecord{}, err
	}

	return record, nil
}

func (s *PostgresStore) CreateAPIKey(ctx context.Context, id uuid.UUID, keyHash string) error {
	_, err := s.db.Exec(
		ctx,
		`INSERT INTO api_keys (id ,key_hash)
		VALUES ($1,$2)`,
		id,
		keyHash,
	)
	return err
}

func (s *PostgresStore) FindAPIKey(ctx context.Context, keyHash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(
		ctx,
		`SELECT id FROM api_keys
		WHERE key_hash = $1`,
		keyHash,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

func (s *PostgresStore) SweepExpired(ctx context.Context) error {
	_, err := s.db.Exec(
		ctx,
		`DELETE FROM urls
		WHERE expires_at <= now()`,
	)
	return err
}
