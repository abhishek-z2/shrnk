package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/abhishek-z2/shrnk/internal/auth"
	"github.com/abhishek-z2/shrnk/internal/cache"
	"github.com/abhishek-z2/shrnk/internal/handlers"
	"github.com/abhishek-z2/shrnk/internal/ratelimit"
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

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	db := store.NewPostgresStore(pool)
	rdb := cache.NewRedisCache(redisClient)
	h := handlers.NewHandler(db, rdb)
	r := chi.NewRouter()
	l := ratelimit.NewRateLimiter(redisClient)

	r.With(
		auth.Middleware(db),
		ratelimit.Middleware(l),
	).Post("/api/shorten", h.Shorten)

	r.Get("/{code}", h.Redirect)

	log.Println("server listening on :8080")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}

}
