package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	// Save original env vars
	origHost := os.Getenv("DB_HOST")
	origPort := os.Getenv("DB_PORT")
	origUser := os.Getenv("DB_USER")
	origPass := os.Getenv("DB_PASSWORD")
	origDB := os.Getenv("DB_NAME")
	origSSL := os.Getenv("DB_SSLMODE")

	// Clean up env vars after test
	defer func() {
		os.Setenv("DB_HOST", origHost)
		os.Setenv("DB_PORT", origPort)
		os.Setenv("DB_USER", origUser)
		os.Setenv("DB_PASSWORD", origPass)
		os.Setenv("DB_NAME", origDB)
		os.Setenv("DB_SSLMODE", origSSL)
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name: "Default values",
			envVars: map[string]string{
				"DB_HOST":     "",
				"DB_PORT":     "",
				"DB_USER":     "",
				"DB_PASSWORD": "",
				"DB_NAME":     "",
				"DB_SSLMODE":  "",
			},
			expected: &Config{
				Host:     "localhost",
				Port:     "5432",
				User:     "postgres",
				Password: "postgres",
				DBName:   "queuet",
				SSLMode:  "disable",
			},
		},
		{
			name: "Custom values",
			envVars: map[string]string{
				"DB_HOST":     "testhost",
				"DB_PORT":     "5433",
				"DB_USER":     "testuser",
				"DB_PASSWORD": "testpass",
				"DB_NAME":     "testdb",
				"DB_SSLMODE":  "require",
			},
			expected: &Config{
				Host:     "testhost",
				Port:     "5433",
				User:     "testuser",
				Password: "testpass",
				DBName:   "testdb",
				SSLMode:  "require",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars for test
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			config := NewConfig()
			assert.Equal(t, tt.expected, config)
		})
	}
}

func TestConnect(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		shouldError bool
	}{
		{
			name: "Invalid connection",
			config: &Config{
				Host:     "nonexistent",
				Port:     "5432",
				User:     "invalid",
				Password: "invalid",
				DBName:   "invalid",
				SSLMode:  "disable",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Connect(tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
				if db != nil {
					db.Close()
				}
			}
		})
	}
}
