package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/abhishek-z2/shrnk/internal/auth"
	"github.com/abhishek-z2/shrnk/internal/cache"
	"github.com/abhishek-z2/shrnk/internal/handlers"
	"github.com/abhishek-z2/shrnk/internal/ratelimit"
	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/abhishek-z2/shrnk/internal/sweeper"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)

	defer stop()

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
	s := sweeper.New(db)

	r.With(
		auth.Middleware(db),
		ratelimit.Middleware(l),
	).Post("/api/shorten", h.Shorten)

	r.Get("/{code}", h.Redirect)

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go s.Run(ctx)

	go func() {
		log.Println("server listening on :8080")
		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
