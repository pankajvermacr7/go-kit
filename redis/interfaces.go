package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// Cache defines an interface for Redis operations.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error)
}
