# API Documentation

## Overview

This document provides comprehensive API documentation for GoKit, a collection of enterprise-grade utilities for database operations, caching, message queuing, and observability.

## Table of Contents

- [PostgreSQL Package](#postgresql-package)
- [Redis Package](#redis-package)
- [Kafka Package](#kafka-package)
- [Logging Package](#logging-package)
- [Sentry Package](#sentry-package)
- [HTTP Package](#http-package)

## PostgreSQL Package

### Configuration

```go
type DBConfig struct {
    Host            string        // Database host
    Port            int           // Database port
    User            string        // Database user
    Password        string        // Database password
    DBName          string        // Database name
    MaxConns        int           // Maximum connections
    MinConns        int           // Minimum connections
    MaxConnLifetime time.Duration // Maximum connection lifetime
    MaxConnIdleTime time.Duration // Maximum idle time
}
```

### Core Functions

#### Connect(cfg DBConfig) (*gorm.DB, error)

Establishes a connection to PostgreSQL database.

```go
config := postgres.DBConfig{
    Host:            "localhost",
    Port:            5432,
    User:            "postgres",
    Password:        "password",
    DBName:          "myapp",
    MaxConns:        10,
    MinConns:        2,
    MaxConnLifetime: 5 * time.Minute,
    MaxConnIdleTime: 5 * time.Minute,
}

db, err := postgres.Connect(config)
if err != nil {
    log.Fatal(err)
}
```

#### RunMigrations(cfg DBConfig, migrationsDir string) error

Applies database migrations from the specified directory.

```go
err := postgres.RunMigrations(config, "./migrations")
if err != nil {
    log.Fatal(err)
}
```

#### CreateTable(ctx context.Context, model interface{}, tableName ...string) error

Creates a table based on the given model.

```go
type User struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"type:varchar(100)"`
    Email string `gorm:"type:varchar(100);uniqueIndex"`
}

ctx := context.Background()
err := postgres.CreateTable(ctx, &User{})
if err != nil {
    log.Fatal(err)
}
```

#### InsertData(ctx context.Context, record interface{}) error

Inserts a single record into the database.

```go
user := &User{Name: "John Doe", Email: "john@example.com"}
err := postgres.InsertData(ctx, user)
if err != nil {
    log.Fatal(err)
}
```

#### FetchData(ctx context.Context, model interface{}, conditions map[string]interface{}) error

Fetches data based on conditions.

```go
var users []User
conditions := map[string]interface{}{"name": "John Doe"}
err := postgres.FetchData(ctx, &users, conditions)
if err != nil {
    log.Fatal(err)
}
```

#### UpdateData(ctx context.Context, model interface{}, updates map[string]interface{}, conditions map[string]interface{}) error

Updates records based on conditions.

```go
updates := map[string]interface{}{"name": "Jane Doe"}
conditions := map[string]interface{}{"email": "john@example.com"}
err := postgres.UpdateData(ctx, &User{}, updates, conditions)
if err != nil {
    log.Fatal(err)
}
```

#### DeleteData(ctx context.Context, model interface{}, conditions map[string]interface{}) error

Deletes records based on conditions.

```go
conditions := map[string]interface{}{"email": "john@example.com"}
err := postgres.DeleteData(ctx, &User{}, conditions)
if err != nil {
    log.Fatal(err)
}
```

#### BatchInsertData(ctx context.Context, records interface{}, batchSize int, opts ...ConflictOption) error

Performs batch insert operations with conflict handling.

```go
users := []User{
    {Name: "User1", Email: "user1@example.com"},
    {Name: "User2", Email: "user2@example.com"},
}

// Do nothing on conflict
err := postgres.BatchInsertData(ctx, users, 100, 
    postgres.WithOnConflictDoNothing([]string{"email"}))

// Update on conflict
err = postgres.BatchInsertData(ctx, users, 100,
    postgres.WithOnConflictDoUpdate([]string{"email"}, 
        map[string]interface{}{"name": "Updated Name"}))
```

## Redis Package

### Configuration

```go
type Config struct {
    Address      string        // Redis server address
    Password     string        // Redis password
    DB           int           // Redis database number
    PoolSize     int           // Connection pool size
    MinIdleConns int           // Minimum idle connections
    DefaultTTL   time.Duration // Default TTL for keys
}
```

### Core Methods

#### NewRedisClient(config Config) (*RedisClient, error)

Creates a new Redis client instance.

```go
config := redis.Config{
    Address:      "localhost:6379",
    Password:     "",
    DB:           0,
    PoolSize:     10,
    MinIdleConns: 2,
    DefaultTTL:   1 * time.Hour,
}

client, err := redis.NewRedisClient(config)
if err != nil {
    log.Fatal(err)
}
```

#### Get(ctx context.Context, key string) (string, error)

Retrieves a value from Redis.

```go
value, err := client.Get(ctx, "user:123")
if err != nil {
    log.Fatal(err)
}
```

#### Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

Sets a value in Redis with optional expiration.

```go
err := client.Set(ctx, "user:123", "user_data", 30*time.Minute)
if err != nil {
    log.Fatal(err)
}
```

#### ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)

Retrieves elements from a sorted set in descending order.

```go
elements, err := client.ZRevRange(ctx, "leaderboard", 0, 9)
if err != nil {
    log.Fatal(err)
}
```

#### ZAdd(ctx context.Context, key string, members ...*redis.Z) (int64, error)

Adds members to a sorted set.

```go
members := []*redis.Z{
    {Score: 100, Member: "player1"},
    {Score: 200, Member: "player2"},
}
count, err := client.ZAdd(ctx, "leaderboard", members...)
if err != nil {
    log.Fatal(err)
}
```

## Kafka Package

### Configuration

```go
type KafkaConfig struct {
    Brokers []string // Kafka broker addresses
    Version string   // Kafka version
}
```

### Producer

#### NewProducer(config *KafkaConfig, encoder Encoder, opts ProducerOptions) (*Producer, error)

Creates a new Kafka producer.

```go
config := &kafka.KafkaConfig{
    Brokers: []string{"localhost:9092"},
    Version: "2.8.0",
}

encoder := kafka.NewJSONEncoder()
producer, err := kafka.NewProducer(config, encoder, kafka.ProducerOptions{
    Async: true,
    ErrorHandler: func(err error) {
        log.Printf("Kafka error: %v", err)
    },
})
if err != nil {
    log.Fatal(err)
}
defer producer.Close()
```

#### SendMessage(topic string, key string, value interface{}, headers map[string]string) error

Sends a message to a Kafka topic.

```go
err := producer.SendMessage("user-events", "user-123", map[string]interface{}{
    "action": "user_created",
    "user_id": "123",
}, map[string]string{
    "source": "api",
})
if err != nil {
    log.Fatal(err)
}
```

### Consumer

#### NewConsumer(config *KafkaConfig, encoder Encoder, opts ConsumerOptions) (*Consumer, error)

Creates a new Kafka consumer.

```go
consumer, err := kafka.NewConsumer(config, encoder, kafka.ConsumerOptions{
    GroupID: "my-group",
    Topics:  []string{"user-events"},
    Handler: func(msg *sarama.ConsumerMessage) error {
        // Process message
        return nil
    },
})
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()
```

## Logging Package

### NewLogger() *zerolog.Logger

Creates a new structured logger instance.

```go
logger := logging.NewLogger()
logger.Info().Str("user_id", "123").Msg("User logged in")
```

## Sentry Package

### Configuration

```go
type Config struct {
    DSN         string  // Sentry DSN
    Environment string  // Environment name
    Release     string  // Release version
    Debug       bool    // Enable debug mode
}
```

### Initialize(config Config) error

Initializes Sentry for error tracking.

```go
config := sentry.Config{
    DSN:         "https://your-dsn@sentry.io/project",
    Environment: "production",
    Release:     "1.0.0",
    Debug:       false,
}

err := sentry.Initialize(config)
if err != nil {
    log.Fatal(err)
}
```

## HTTP Package

### Client

#### NewClient(config ClientConfig) (*Client, error)

Creates a new HTTP client with configuration.

```go
config := http.ClientConfig{
    Timeout: 30 * time.Second,
    Retries: 3,
}

client, err := http.NewClient(config)
if err != nil {
    log.Fatal(err)
}
```

#### Get(ctx context.Context, url string, headers map[string]string) (*Response, error)

Performs an HTTP GET request.

```go
headers := map[string]string{
    "Authorization": "Bearer token",
}
response, err := client.Get(ctx, "https://api.example.com/users", headers)
if err != nil {
    log.Fatal(err)
}
```

#### Post(ctx context.Context, url string, body interface{}, headers map[string]string) (*Response, error)

Performs an HTTP POST request.

```go
data := map[string]interface{}{
    "name": "John Doe",
    "email": "john@example.com",
}
response, err := client.Post(ctx, "https://api.example.com/users", data, headers)
if err != nil {
    log.Fatal(err)
}
```

## Error Handling

All packages use consistent error handling patterns:

```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

## Context Usage

All operations support context for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := postgres.InsertData(ctx, record)
```

## Best Practices

1. **Always use context**: Pass context to all operations for proper cancellation and timeout handling.
2. **Handle errors properly**: Check and handle errors at each step.
3. **Use connection pooling**: Configure appropriate pool sizes for your workload.
4. **Implement retry logic**: Use exponential backoff for transient failures.
5. **Monitor performance**: Use the built-in metrics for monitoring.
6. **Secure configuration**: Use environment variables for sensitive configuration.
7. **Test thoroughly**: Use the provided integration tests for comprehensive testing.

## Examples

See the [examples](./examples) directory for complete working examples of each package. 