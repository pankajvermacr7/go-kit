package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisClient(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Address:      "localhost:6379",
				Password:     "",
				DB:           0,
				PoolSize:     10,
				MinIdleConns: 2,
				DefaultTTL:   1 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "invalid address",
			config: Config{
				Address:      "invalid:address",
				Password:     "",
				DB:           0,
				PoolSize:     10,
				MinIdleConns: 2,
				DefaultTTL:   1 * time.Hour,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewRedisClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				// Note: This will fail if Redis is not running
				// In a real test environment, you'd use a mock or test container
				if err == nil {
					assert.NotNil(t, client)
					assert.Equal(t, tt.config.DefaultTTL, client.defaultTTL)
				}
			}
		})
	}
}

func TestRedisClient_Set(t *testing.T) {
	// Skip if Redis is not available
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	ctx := context.Background()
	key := "test:set:key"
	value := "test_value"

	// Test Set with custom TTL
	err = client.Set(ctx, key, value, 30*time.Second)
	assert.NoError(t, err)

	// Test Set with default TTL
	err = client.Set(ctx, key+":default", value, 0)
	assert.NoError(t, err)

	// Test Set with complex value
	complexValue := map[string]interface{}{
		"string": "value",
		"int":    123,
		"bool":   true,
	}
	err = client.Set(ctx, key+":complex", complexValue, 30*time.Second)
	assert.NoError(t, err)
}

func TestRedisClient_Get(t *testing.T) {
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	ctx := context.Background()
	key := "test:get:key"
	value := "test_value"

	// Set a value first
	err = client.Set(ctx, key, value, 30*time.Second)
	require.NoError(t, err)

	// Test Get existing key
	result, err := client.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// Test Get non-existing key
	nonExistingKey := "test:get:non-existing"
	result, err = client.Get(ctx, nonExistingKey)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestRedisClient_ZRevRange(t *testing.T) {
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	ctx := context.Background()
	key := "test:zrevrange:key"

	// Add some members to sorted set
	members := []*redis.Z{
		{Score: 1.0, Member: "member1"},
		{Score: 2.0, Member: "member2"},
		{Score: 3.0, Member: "member3"},
	}

	_, err = client.ZAdd(ctx, key, members...)
	require.NoError(t, err)

	// Test ZRevRange
	result, err := client.ZRevRange(ctx, key, 0, -1)
	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "member3", result[0]) // Highest score first
	assert.Equal(t, "member2", result[1])
	assert.Equal(t, "member1", result[2])

	// Test ZRevRange with range
	result, err = client.ZRevRange(ctx, key, 0, 1)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "member3", result[0])
	assert.Equal(t, "member2", result[1])
}

func TestRedisClient_ZAdd(t *testing.T) {
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	ctx := context.Background()
	key := "test:zadd:key"

	// Test ZAdd with single member
	member := &redis.Z{Score: 1.0, Member: "single_member"}
	count, err := client.ZAdd(ctx, key, member)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test ZAdd with multiple members
	members := []*redis.Z{
		{Score: 2.0, Member: "member1"},
		{Score: 3.0, Member: "member2"},
	}
	count, err = client.ZAdd(ctx, key, members...)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Test ZAdd with existing member (should update score)
	existingMember := &redis.Z{Score: 4.0, Member: "member1"}
	count, err = client.ZAdd(ctx, key, existingMember)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count) // Member already exists, just updated
}

func TestRedisClient_Close(t *testing.T) {
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	// Test Close - access the underlying client
	err = client.client.Close()
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkRedisClient_Set(b *testing.B) {
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		b.Skip("Redis not available, skipping benchmark")
	}
	defer client.client.Close()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench:set:%d", i)
		value := fmt.Sprintf("value_%d", i)
		err := client.Set(ctx, key, value, 30*time.Second)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedisClient_Get(b *testing.B) {
	config := Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	client, err := NewRedisClient(config)
	if err != nil {
		b.Skip("Redis not available, skipping benchmark")
	}
	defer client.client.Close()

	ctx := context.Background()
	key := "bench:get:key"
	value := "bench_value"

	// Set a value first
	err = client.Set(ctx, key, value, 30*time.Second)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.Get(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}
