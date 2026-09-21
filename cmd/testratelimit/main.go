package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/abhishek-z2/shrnk/internal/ratelimit"
)

func main() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	limiter := ratelimit.NewRateLimiter(client)

	key := "test-rate-limit"

	for i := 1; i <= 7; i++ {
		allowed, err := limiter.Allow(ctx, key, 5, 1)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("request %d: allowed=%v\n", i, allowed)
	}

	fmt.Println("waiting 2 seconds...")
	time.Sleep(2 * time.Second)

	allowed, err := limiter.Allow(ctx, key, 5, 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("after waiting: allowed=%v\n", allowed)
}
