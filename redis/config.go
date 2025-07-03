package redis

import "time"

// Config holds the Redis configuration details.
type Config struct {
	Address      string        // Redis server address (host:port)
	Password     string        // Password for Redis authentication
	DB           int           // Database index
	PoolSize     int           // Maximum number of connections in the pool
	MinIdleConns int           // Minimum number of idle connections
	DefaultTTL   time.Duration // Default expiration time for keys
}
