package store

import (
	"context"
	"errors"
	//"log"

	"github.com/abhishek-z2/shrnk/internal/shortcode"
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

func (s *PostgresStore) CreateURL(ctx context.Context, longURL string) (int64, string, error) {
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
		`INSERT INTO urls(id,short_code,long_url)
		VALUES ($1,$2,$3)`,
		id,
		shortcode,
		longURL,
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

func (s *PostgresStore) GetURL(ctx context.Context, shortCode string) (string, error) {
	var longURL string

	err := s.db.QueryRow(
		ctx,
		`SELECT long_url
		FROM urls 
		WHERE short_code=($1)`,
		shortCode,
	).Scan(&longURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	return longURL, nil
}
