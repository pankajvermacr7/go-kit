# GoKit - Enterprise Go Utilities

[![Go Report Card](https://goreportcard.com/badge/github.com/pankajvermacr7/gokit)](https://goreportcard.com/report/github.com/pankajvermacr7/gokit)
[![Go Version](https://img.shields.io/github/go-mod/go-version/pankajvermacr7/gokit)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](https://github.com/pankajvermacr7/gokit/actions)

A comprehensive Go toolkit providing enterprise-grade utilities for database operations, caching, message queuing, and observability. Built with production-ready features including connection pooling, metrics, error handling, and comprehensive testing.

## 🚀 Features

### Database Management
- **PostgreSQL Integration**: Full GORM support with connection pooling and migration management
- **Redis Caching**: High-performance caching with TTL support and sorted sets
- **Connection Pooling**: Configurable connection pools for optimal performance
- **Migration System**: Automated database schema management

### Message Queuing
- **Kafka Producer/Consumer**: Async and sync message publishing with custom encoders
- **Error Handling**: Robust error handling with retry mechanisms
- **Metrics**: Built-in Prometheus metrics for monitoring

### Observability & Monitoring
- **Structured Logging**: ZeroLog-based logging with configurable levels
- **Sentry Integration**: Error tracking and performance monitoring
- **Prometheus Metrics**: Custom metrics for database and message queue operations
- **OpenTelemetry**: Distributed tracing support

### Development & Testing
- **Integration Tests**: Comprehensive test suite using testcontainers
- **Docker Support**: Containerized testing environment
- **CI/CD Ready**: GitHub Actions workflow templates

## 📦 Installation

```bash
go get github.com/pankajvermacr7/gokit
```

## 🛠️ Quick Start

### PostgreSQL Setup

```go
package main

import (
    "context"
    "time"
    "github.com/pankajvermacr7/gokit/postgres"
)

func main() {
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

    // Connect to database
    db, err := postgres.Connect(config)
    if err != nil {
        log.Fatal(err)
    }

    // Run migrations
    err = postgres.RunMigrations(config, "./migrations")
    if err != nil {
        log.Fatal(err)
    }

    // Create table
    ctx := context.Background()
    err = postgres.CreateTable(ctx, &User{})
    if err != nil {
        log.Fatal(err)
    }
}
```

### Redis Caching

```go
package main

import (
    "context"
    "time"
    "github.com/pankajvermacr7/gokit/redis"
)

func main() {
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

    ctx := context.Background()
    
    // Set cache
    err = client.Set(ctx, "user:123", "user_data", 30*time.Minute)
    if err != nil {
        log.Fatal(err)
    }

    // Get cache
    value, err := client.Get(ctx, "user:123")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Kafka Messaging

```go
package main

import (
    "github.com/pankajvermacr7/gokit/kafka"
)

func main() {
    config := kafka.DefaultConfig()
    config.Brokers = []string{"localhost:9092"}

    encoder := kafka.JSONEncoder{}
    
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

    // Send message
    err = producer.SendMessage("user-events", "user-123", map[string]interface{}{
        "action": "user_created",
        "user_id": "123",
    }, map[string]string{
        "source": "api",
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

## 🧪 Testing

Run the complete test suite:

```bash
# Run all tests
go test ./...

# Run integration tests (requires Docker)
go test -tags=integration ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```

## 📊 Performance

The toolkit is optimized for high-performance applications:

- **Connection Pooling**: Configurable pool sizes for optimal resource usage
- **Batch Operations**: Efficient batch inserts and updates
- **Async Processing**: Non-blocking Kafka operations
- **Caching Strategies**: TTL-based caching with Redis

## 🏗️ Architecture

```
gokit/
├── postgres/          # PostgreSQL operations and migrations
├── redis/            # Redis caching and operations
├── kafka/            # Kafka producer/consumer
├── logging/          # Structured logging
├── sentry/           # Error tracking
├── http/             # HTTP client utilities
└── pgx/              # Low-level PostgreSQL operations
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🚀 Roadmap

- [ ] GraphQL support
- [ ] MongoDB integration
- [ ] Elasticsearch client
- [ ] gRPC utilities
- [ ] WebSocket support
- [ ] Rate limiting utilities
- [ ] Circuit breaker pattern
- [ ] Distributed locking

## 📞 Support

For support and questions, please open an issue on GitHub or contact the maintainers.

---

**Built with ❤️ for the Go community**
