package redis

import (
	"context"
	"fmt"
	"time"
)

// Exists checks if a key exists in Redis.
func (rc *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	count, err := rc.client.Exists(ctx, key).Result()
	if err != nil {
		return false, WrapError("checking key existence", key, err)
	}
	return count > 0, nil
}

// Delete removes one or more keys from Redis.
func (rc *RedisClient) Delete(ctx context.Context, keys ...string) (int64, error) {
	deleted, err := rc.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, WrapError("deleting keys", "", err)
	}
	return deleted, nil
}

// Expire sets an expiration time for a key.
func (rc *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	success, err := rc.client.Expire(ctx, key, expiration).Result()
	if err != nil {
		return false, WrapError("setting expiration", key, err)
	}
	return success, nil
}

// TTL retrieves the remaining time-to-live (TTL) for a key.
func (rc *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := rc.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, WrapError("retrieving TTL", key, err)
	}
	return ttl, nil
}

// Increment increases the value of a key by the given amount. Initializes the key if it doesn't exist.
func (rc *RedisClient) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	val, err := rc.client.IncrBy(ctx, key, amount).Result()
	if err != nil {
		return 0, WrapError("incrementing key", key, err)
	}
	return val, nil
}

// Decrement decreases the value of a key by the given amount. Initializes the key if it doesn't exist.
func (rc *RedisClient) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	val, err := rc.client.DecrBy(ctx, key, amount).Result()
	if err != nil {
		return 0, WrapError("decrementing key", key, err)
	}
	return val, nil
}

// Keys retrieves all keys matching a specific pattern.
func (rc *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	keys, err := rc.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, WrapError("retrieving keys", pattern, err)
	}
	return keys, nil
}

// FlushDB deletes all keys in the currently selected Redis database.
func (rc *RedisClient) FlushDB(ctx context.Context) error {
	if err := rc.client.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush database: %w", err)
	}
	return nil
}

// HGetAll retrieves all fields and values from a hash stored at a key.
func (rc *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	values, err := rc.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, WrapError("retrieving hash values", key, err)
	}
	return values, nil
}
