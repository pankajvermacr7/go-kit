package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pankajvermacr7/gokit/logging"
)

// RedisClient is an implementation of the Cache interface.
type RedisClient struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewRedisClient initializes a new Redis client with the given configuration.
func NewRedisClient(config Config) (*RedisClient, error) {
	logger := logging.NewLogger()
	client := redis.NewClient(&redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
	})

	// Test the connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info().Msg("Connected to Redis")

	return &RedisClient{
		client:     client,
		defaultTTL: config.DefaultTTL,
	}, nil
}

// Get retrieves the value of a key from Redis.
func (rc *RedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := rc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key does not exist
	}
	if err != nil {
		return "", WrapError("GET", key, err)
	}
	return val, nil
}

// Set sets a value for a key in Redis with an optional expiration time.
func (rc *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if expiration == 0 {
		expiration = rc.defaultTTL
	}
	if err := rc.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return WrapError("SET", key, err)
	}
	return nil
}

// ZRevRange retrieves elements from a sorted set in descending order.
func (rc *RedisClient) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	vals, err := rc.client.ZRevRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, WrapError("ZRevRange", key, err)
	}
	return vals, nil
}

// ZAdd adds one or more members to a sorted set or updates their scores.
func (rc *RedisClient) ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error) {
	n, err := rc.client.ZAdd(ctx, key, members...).Result()
	if err != nil {
		return 0, WrapError("ZADD", key, err)
	}
	return n, nil
}
