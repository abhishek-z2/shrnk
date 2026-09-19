package store

import (
	"context"
	"github.com/abhishek-z2/shrnk/internal/shortcode"
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
