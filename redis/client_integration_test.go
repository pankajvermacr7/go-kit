package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testConfig Config

func setupRedisContainer(t *testing.T) (testcontainers.Container, string) {
	t.Helper() // Mark as a helper function for better error reporting
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}

	ip, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get Redis container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("Failed to get Redis container port: %v", err)
	}

	return container, ip + ":" + port.Port()
}

func setupTest(t *testing.T) (testcontainers.Container, *RedisClient) {
	container, address := setupRedisContainer(t)

	testConfig = Config{
		Address:      address,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   5 * time.Minute,
	}

	client, err := NewRedisClient(testConfig)
	if err != nil {
		container.Terminate(context.Background())
		t.Fatalf("Failed to initialize Redis client: %v", err)
	}

	return container, client
}

func teardownTest(container testcontainers.Container) {
	container.Terminate(context.Background())
}

func TestRedisIntegration(t *testing.T) {
	container, client := setupTest(t)
	defer teardownTest(container)

	t.Run("TestNewRedisClient", func(t *testing.T) {
		assert.NotNil(t, client)
	})

	t.Run("TestRedisClient_GetSet", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key"
		value := "test_value"

		err := client.Set(ctx, key, value, 0)
		assert.NoError(t, err)

		got, err := client.Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, value, got)
	})

	t.Run("TestRedisClient_Exists", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key_exists"
		value := "test_value"

		err := client.Set(ctx, key, value, 0)
		assert.NoError(t, err)

		exists, err := client.Exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("TestRedisClient_Delete", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key_delete"
		value := "test_value"

		err := client.Set(ctx, key, value, 0)
		assert.NoError(t, err)

		_, err = client.Delete(ctx, key)
		assert.NoError(t, err)

		exists, err := client.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("TestRedisClient_Expire", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key_expire"
		value := "test_value"

		err := client.Set(ctx, key, value, 0)
		assert.NoError(t, err)

		ok, err := client.Expire(ctx, key, 1*time.Second)
		assert.True(t, ok)
		assert.NoError(t, err)

		time.Sleep(2 * time.Second)

		exists, err := client.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("TestRedisClient_TTL", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key_ttl"
		value := "test_value"

		err := client.Set(ctx, key, value, 10*time.Second)
		assert.NoError(t, err)

		ttl, err := client.TTL(ctx, key)
		assert.NoError(t, err)
		assert.True(t, ttl > 0)
	})

	t.Run("TestRedisClient_Incr", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key_incr"

		err := client.Set(ctx, key, "0", 0)
		assert.NoError(t, err)

		val, err := client.Increment(ctx, key, 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), val)
	})

	t.Run("TestRedisClient_Decr", func(t *testing.T) {
		ctx := context.Background()
		key := "test_key_decr"

		err := client.Set(ctx, key, "0", 0)
		assert.NoError(t, err)

		val, err := client.Decrement(ctx, key, 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(-1), val)
	})
}
