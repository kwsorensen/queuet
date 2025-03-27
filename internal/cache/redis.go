package cache

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     int
	Password string
}

// NewRedisConfig creates a new Redis configuration from environment variables
func NewRedisConfig() *RedisConfig {
	port, _ := strconv.Atoi(getEnv("REDIS_PORT", "6379"))
	return &RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     port,
		Password: getEnv("REDIS_PASSWORD", ""),
	}
}

// NewRedisClient creates a new Redis client
func NewRedisClient(config *RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       0,
	})

	// Test the connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("error connecting to Redis: %v", err)
	}

	return client, nil
}

// getEnv retrieves an environment variable with a fallback value
func getEnv(key, fallback string) string {
	val, exists := os.LookupEnv(key)
	if !exists || val == "" {
		return fallback
	}
	return val
}
