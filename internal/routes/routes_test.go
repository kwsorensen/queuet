package routes

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/queuet/internal/handlers"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// Mock Redis client
type redisMock struct {
	getFunc func(ctx context.Context, key string) *redis.StringCmd
	setFunc func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	delFunc func(ctx context.Context, keys ...string) *redis.IntCmd
}

func (m *redisMock) Get(ctx context.Context, key string) *redis.StringCmd {
	if m.getFunc != nil {
		return m.getFunc(ctx, key)
	}
	return redis.NewStringCmd(ctx)
}

func (m *redisMock) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if m.setFunc != nil {
		return m.setFunc(ctx, key, value, expiration)
	}
	return redis.NewStatusCmd(ctx)
}

func (m *redisMock) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if m.delFunc != nil {
		return m.delFunc(ctx, keys...)
	}
	return redis.NewIntCmd(ctx)
}

func TestSetupRoutes(t *testing.T) {
	// Create mock DB
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	// Create mock Redis client
	redisClient := &redisMock{
		getFunc: func(ctx context.Context, key string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx)
			cmd.SetErr(redis.Nil)
			return cmd
		},
	}

	// Create task handler with mocks
	taskHandler := handlers.NewTaskHandler(db, redisClient)

	// Create router and register routes
	r := chi.NewRouter()
	SetupRoutes(r, taskHandler)

	// Test cases for different routes
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		mockDB         func()
	}{
		{
			name:           "GET /tasks",
			method:         "GET",
			path:           "/api/v1/tasks",
			expectedStatus: http.StatusOK,
			mockDB: func() {
				mock.ExpectQuery(`SELECT id, title, description, status, created_at, updated_at FROM tasks ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "created_at", "updated_at"}).
						AddRow(1, "Task 1", "Description 1", "pending", time.Now(), time.Now()))
			},
		},
		{
			name:           "GET /tasks/{id}",
			method:         "GET",
			path:           "/api/v1/tasks/1",
			expectedStatus: http.StatusNotFound,
			mockDB: func() {
				mock.ExpectQuery(`SELECT id, title, description, status, created_at, updated_at FROM tasks WHERE id = \$1`).
					WithArgs(1).
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:           "DELETE /tasks/{id}",
			method:         "DELETE",
			path:           "/api/v1/tasks/1",
			expectedStatus: http.StatusNotFound,
			mockDB: func() {
				mock.ExpectExec(`DELETE FROM tasks WHERE id = \$1`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockDB()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
