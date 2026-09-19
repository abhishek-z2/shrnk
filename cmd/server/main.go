package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abhishek-z2/shrnk/internal/handlers"
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

	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	db := store.NewPostgresStore(pool)
	h := handlers.NewHandler(db)

	r := chi.NewRouter()

	r.Post("/api/shorten", h.Shorten)
	r.Get("/{code}", h.Redirect)

	log.Println("server listening on :8080")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}

}
