package service

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter is a fixed-window counter: SET NX with the window TTL, then INCR (keeps the TTL), in one
// MULTI, so the window starts at the first attempt.
type RedisLimiter struct {
	rdb    *redis.Client
	prefix string
}

// DefaultLimiterPrefix namespaces the login rate-limit keys in Redis.
const DefaultLimiterPrefix = "sentra:auth:rl:"

// NewRedisLimiter creates a limiter backed by rdb; every key is prefixed with prefix.
func NewRedisLimiter(rdb *redis.Client, prefix string) *RedisLimiter {
	return &RedisLimiter{rdb: rdb, prefix: prefix}
}

// Hit implements Limiter.
func (l *RedisLimiter) Hit(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	key = l.prefix + key
	pipe := l.rdb.TxPipeline()
	pipe.SetNX(ctx, key, 0, window)
	incr := pipe.Incr(ctx, key)
	ttl := pipe.PTTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}
	if incr.Val() > int64(limit) {
		wait := ttl.Val()
		if wait <= 0 {
			wait = window
		}
		return false, wait, nil
	}
	return true, 0, nil
}
