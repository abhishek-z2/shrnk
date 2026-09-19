package main

import (
	"context"
	"fmt"
	"log"

	"github.com/abhishek-z2/shrnk/internal/cache"
)

func main() {
	ctx := context.Background()

	rdb := cache.NewRedisCache()
	err := rdb.Set(ctx, "test", "hello redis")
	if err != nil {
		log.Fatal(err)
	}

	value, err := rdb.Get(ctx, "test")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(value)
}
