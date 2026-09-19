package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abhishek-z2/shrnk/internal/store"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(
		ctx,
		"postgres://postgres:postgres@localhost:5433/shrnk",
	)

	if err != nil {
		log.Fatal(err)
	}

	defer pool.Close()

	err = pool.Ping(ctx)
	if err != nil {
		log.Fatal(err)
	}

	db := store.NewPostgresStore(pool)

	id, code, err := db.CreateURL(
		ctx,
		"https://example.com",
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("id=%d code=%s\n", id, code)
}
