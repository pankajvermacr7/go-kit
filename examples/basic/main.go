package main

import (
	"context"
	"log"
	"time"

	"github.com/pankajvermacr7/gokit/kafka"
	"github.com/pankajvermacr7/gokit/postgres"
	"github.com/pankajvermacr7/gokit/redis"
)

// User represents a user in our system
type User struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"type:varchar(100)"`
	Email     string `gorm:"type:varchar(100);uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func main() {
	ctx := context.Background()

	// Initialize PostgreSQL
	postgresConfig := postgres.DBConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "postgres",
		DBName:          "gokit_example",
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: 5 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}

	_, err := postgres.Connect(postgresConfig)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Create table
	err = postgres.CreateTable(ctx, &User{})
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Initialize Redis
	redisConfig := redis.Config{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   1 * time.Hour,
	}

	redisClient, err := redis.NewRedisClient(redisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize Kafka
	kafkaConfig := kafka.DefaultConfig()
	kafkaConfig.Brokers = []string{"localhost:9092"}

	encoder := kafka.JSONEncoder{}
	producer, err := kafka.NewProducer(kafkaConfig, encoder, kafka.ProducerOptions{
		Async: true,
		ErrorHandler: func(err error) {
			log.Printf("Kafka error: %v", err)
		},
	})
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer producer.Close()

	// Example: Create a user
	user := &User{
		Name:  "John Doe",
		Email: "john.doe@example.com",
	}

	// Insert user into database
	err = postgres.InsertData(ctx, user)
	if err != nil {
		log.Fatalf("Failed to insert user: %v", err)
	}

	// Cache user data in Redis
	userKey := "user:" + user.Email
	err = redisClient.Set(ctx, userKey, user, 30*time.Minute)
	if err != nil {
		log.Printf("Failed to cache user: %v", err)
	}

	// Publish user creation event to Kafka
	err = producer.SendMessage("user-events", user.Email, map[string]interface{}{
		"action":    "user_created",
		"user_id":   user.ID,
		"email":     user.Email,
		"name":      user.Name,
		"timestamp": time.Now(),
	}, map[string]string{
		"source": "example-app",
	})
	if err != nil {
		log.Printf("Failed to publish event: %v", err)
	}

	// Example: Retrieve user from cache first, then database
	cachedUser, err := redisClient.Get(ctx, userKey)
	if err != nil {
		log.Printf("Failed to get from cache: %v", err)
	}

	if cachedUser == "" {
		// Cache miss, get from database
		var users []User
		err = postgres.FetchData(ctx, &users, map[string]interface{}{
			"email": user.Email,
		})
		if err != nil {
			log.Fatalf("Failed to fetch user: %v", err)
		}

		if len(users) > 0 {
			// Cache the result
			err = redisClient.Set(ctx, userKey, users[0], 30*time.Minute)
			if err != nil {
				log.Printf("Failed to cache user: %v", err)
			}
			log.Printf("User retrieved from database: %+v", users[0])
		}
	} else {
		log.Printf("User retrieved from cache: %s", cachedUser)
	}

	// Example: Update user
	updates := map[string]interface{}{
		"name": "John Updated Doe",
	}
	conditions := map[string]interface{}{
		"email": user.Email,
	}

	err = postgres.UpdateData(ctx, &User{}, updates, conditions)
	if err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}

	// Invalidate cache
	err = redisClient.Set(ctx, userKey, "", 1*time.Second) // Set to expire immediately
	if err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}

	// Publish update event
	err = producer.SendMessage("user-events", user.Email, map[string]interface{}{
		"action":    "user_updated",
		"user_id":   user.ID,
		"email":     user.Email,
		"changes":   updates,
		"timestamp": time.Now(),
	}, map[string]string{
		"source": "example-app",
	})
	if err != nil {
		log.Printf("Failed to publish update event: %v", err)
	}

	log.Println("Example completed successfully!")
}
