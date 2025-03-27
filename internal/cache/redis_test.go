package cache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRedisConfig(t *testing.T) {
	// Save original env vars
	origHost := os.Getenv("REDIS_HOST")
	origPort := os.Getenv("REDIS_PORT")
	origPassword := os.Getenv("REDIS_PASSWORD")

	// Cleanup
	defer func() {
		os.Setenv("REDIS_HOST", origHost)
		os.Setenv("REDIS_PORT", origPort)
		os.Setenv("REDIS_PASSWORD", origPassword)
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected *RedisConfig
	}{
		{
			name: "Default_values",
			envVars: map[string]string{
				"REDIS_HOST":     "",
				"REDIS_PORT":     "",
				"REDIS_PASSWORD": "",
			},
			expected: &RedisConfig{
				Host:     "localhost",
				Port:     6379,
				Password: "",
			},
		},
		{
			name: "Custom_values",
			envVars: map[string]string{
				"REDIS_HOST":     "testhost",
				"REDIS_PORT":     "6380",
				"REDIS_PASSWORD": "testpass",
			},
			expected: &RedisConfig{
				Host:     "testhost",
				Port:     6380,
				Password: "testpass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			config := NewRedisConfig()
			assert.Equal(t, tt.expected, config)
		})
	}
}

func TestNewRedisClient(t *testing.T) {
	t.Run("Invalid_connection", func(t *testing.T) {
		config := &RedisConfig{
			Host:     "nonexistenthost",
			Port:     6379,
			Password: "",
		}

		client, err := NewRedisClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
	})
}
