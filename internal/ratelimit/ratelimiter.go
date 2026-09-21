package ratelimit

import (
	"context"
	_ "embed"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed token_bucket.lua
var tokenBucketScript string

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{
		client: client,
	}
}

func (r *RateLimiter) Allow(
	ctx context.Context,
	key string,
	capacity int,
	refillRate float64,
) (bool, error) {
	now := time.Now().UnixMilli()

	result, err := r.client.Eval(
		ctx,
		tokenBucketScript,
		[]string{key},
		now,
		capacity,
		refillRate,
	).Result()

	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}
