package main

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRateLimiter_TTLLoss(t *testing.T) {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	key := "rate_limit:test_user_test"
	rdb.Del(ctx, key)
	defer rdb.Del(ctx, key)

	err := rdb.Set(ctx, key, "10", 0).Err()
	if err != nil {
		t.Fatalf("Failed to seed key: %v", err)
	}

	limiter := NewRateLimiter(rdb, 10, 10*time.Second)

	allowed, err := limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Error checking rate limit: %v", err)
	}

	if allowed {
		t.Error("Expected request to be blocked, but it was allowed")
	}

	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get TTL: %v", err)
	}

	if ttl <= 0 {
		t.Errorf("Expected positive TTL, got %v", ttl)
	}
}