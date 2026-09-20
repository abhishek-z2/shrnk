package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abhishek-z2/shrnk/internal/auth"
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

	db := store.NewPostgresStore(pool)

	key, err := auth.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	keyHash := auth.HashKey(key)

	id := uuid.New()

	if err := db.CreateAPIKey(ctx, id, keyHash); err != nil {
		log.Fatal(err)
	}

	fmt.Println("API key:", key)
}
