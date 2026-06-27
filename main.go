package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	period time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int, period time.Duration) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		limit:  limit,
		period: period,
	}
}

var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local expire_seconds = tonumber(ARGV[2])

local current = redis.call("INCR", key)
local ttl = redis.call("TTL", key)

if tonumber(current) == 1 or tonumber(ttl) == -1 then
    redis.call("EXPIRE", key, expire_seconds)
end

return {current, ttl}
`)

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	seconds := int(rl.period.Seconds())
	if seconds <= 0 {
		seconds = 1
	}

	res, err := rateLimitScript.Run(ctx, rl.rdb, []string{key}, rl.limit, seconds).Result()
	if err != nil {
		return false, err
	}

	slice, ok := res.([]interface{})
	if !ok || len(slice) < 2 {
		return false, fmt.Errorf("unexpected response format from redis script")
	}

	current := slice[0].(int64)
	return current <= int64(rl.limit), nil
}

func main() {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}

	key := "rate_limit:test_user"
	rdb.Del(ctx, key)

	err := rdb.Set(ctx, key, "10", 0).Err()
	if err != nil {
		fmt.Printf("Failed to seed key: %v\n", err)
		return
	}

	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		fmt.Printf("Failed to get TTL: %v\n", err)
		return
	}
	fmt.Printf("Initial TTL of seeded key: %v\n", ttl)

	limiter := NewRateLimiter(rdb, 10, 10*time.Second)

	allowed, err := limiter.Allow(ctx, key)
	if err != nil {
		fmt.Printf("Error checking rate limit: %v\n", err)
		return
	}

	fmt.Printf("Request allowed: %v (expected: false)\n", allowed)

	newTTL, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		fmt.Printf("Failed to get new TTL: %v\n", err)
		return
	}
	fmt.Printf("New TTL after rate limit check: %v\n", newTTL)

	if newTTL <= 0 {
		fmt.Println("Error: TTL was not restored/set!")
	} else {
		fmt.Println("Success: TTL was successfully restored!")
	}
}