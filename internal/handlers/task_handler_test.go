package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/queuet/internal/models"
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

// Setup test handler with mock DB and Redis
func setupTestHandler(t *testing.T) (*TaskHandler, sqlmock.Sqlmock) {
	// Create mock DB
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}

	// Create mock Redis client
	redisClient := &redisMock{}

	// Create task handler with mocks
	handler := &TaskHandler{
		db:    db,
		cache: redisClient,
	}

	return handler, mock
}

func TestTaskHandler_CreateTask(t *testing.T) {
	handler, mock := setupTestHandler(t)

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		mockDB         func()
	}{
		{
			name:           "Valid request",
			payload:        `{"title": "Test Task", "description": "Test Description"}`,
			expectedStatus: http.StatusCreated,
			mockDB: func() {
				mock.ExpectQuery(`INSERT INTO tasks \(title, description, status, created_at, updated_at\) VALUES \(\$1, \$2, \$3, \$4, \$4\) RETURNING id`).
					WithArgs("Test Task", "Test Description", "pending", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
		},
		{
			name:           "Invalid JSON",
			payload:        `{"title": "Test Task", "description": }`,
			expectedStatus: http.StatusBadRequest,
			mockDB:         func() {},
		},
		{
			name:           "Empty request",
			payload:        `{}`,
			expectedStatus: http.StatusBadRequest,
			mockDB:         func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockDB()

			req := httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader(tt.payload))
			w := httptest.NewRecorder()

			handler.CreateTask(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTaskHandler_GetTask(t *testing.T) {
	handler, mock := setupTestHandler(t)

	tests := []struct {
		name           string
		taskID         string
		expectedStatus int
		checkResponse  bool
		mockDB         func()
		mockRedis      func(h *TaskHandler)
	}{
		{
			name:           "Valid task ID",
			taskID:         "1",
			expectedStatus: http.StatusOK,
			checkResponse:  true,
			mockDB: func() {
				mock.ExpectQuery(`SELECT id, title, description, status, created_at, updated_at FROM tasks WHERE id = \$1`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "created_at", "updated_at"}).
						AddRow(1, "Test Task", "Test Description", "pending", time.Now(), time.Now()))
			},
			mockRedis: func(h *TaskHandler) {
				h.cache.(*redisMock).getFunc = func(ctx context.Context, key string) *redis.StringCmd {
					cmd := redis.NewStringCmd(ctx)
					cmd.SetErr(redis.Nil)
					return cmd
				}
			},
		},
		{
			name:           "Invalid task ID format",
			taskID:         "invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse:  false,
			mockDB:         func() {},
			mockRedis:      func(h *TaskHandler) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockDB()
			tt.mockRedis(handler)

			req := httptest.NewRequest("GET", "/api/v1/tasks/"+tt.taskID, nil)
			w := httptest.NewRecorder()

			// Setup chi router context with URL parameters
			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.taskID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			handler.GetTask(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse {
				var response models.Task
				err := json.NewDecoder(w.Body).Decode(&response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.Title)
				if tt.taskID != "-1" {
					assert.Equal(t, tt.taskID, strconv.FormatInt(response.ID, 10))
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTaskHandler_UpdateTask(t *testing.T) {
	handler, mock := setupTestHandler(t)

	tests := []struct {
		name           string
		taskID         string
		payload        string
		expectedStatus int
		mockDB         func()
	}{
		{
			name:           "Valid update",
			taskID:         "1",
			payload:        `{"title": "Updated Task", "status": "completed"}`,
			expectedStatus: http.StatusOK,
			mockDB: func() {
				mock.ExpectQuery(`UPDATE tasks SET title = COALESCE\(\$1, title\), description = COALESCE\(\$2, description\), status = COALESCE\(\$3, status\), updated_at = \$4 WHERE id = \$5 RETURNING id, title, description, status, created_at, updated_at`).
					WithArgs(
						sql.NullString{String: "Updated Task", Valid: true},
						sql.NullString{String: "", Valid: false},
						sql.NullString{String: "completed", Valid: true},
						sqlmock.AnyArg(),
						1,
					).
					WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "created_at", "updated_at"}).
						AddRow(1, "Updated Task", "Test Description", "completed", time.Now(), time.Now()))
			},
		},
		{
			name:           "Invalid task ID",
			taskID:         "invalid",
			payload:        `{"title": "Updated Task"}`,
			expectedStatus: http.StatusBadRequest,
			mockDB:         func() {},
		},
		{
			name:           "Invalid JSON",
			taskID:         "1",
			payload:        `{"title": "Updated Task", status: }`,
			expectedStatus: http.StatusBadRequest,
			mockDB:         func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockDB()

			req := httptest.NewRequest("PUT", "/api/v1/tasks/"+tt.taskID, strings.NewReader(tt.payload))
			w := httptest.NewRecorder()

			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.taskID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			handler.UpdateTask(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTaskHandler_DeleteTask(t *testing.T) {
	handler, mock := setupTestHandler(t)

	tests := []struct {
		name           string
		taskID         string
		expectedStatus int
		mockDB         func()
	}{
		{
			name:           "Valid delete",
			taskID:         "1",
			expectedStatus: http.StatusNoContent,
			mockDB: func() {
				mock.ExpectExec(`DELETE FROM tasks WHERE id = \$1`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:           "Invalid task ID",
			taskID:         "invalid",
			expectedStatus: http.StatusBadRequest,
			mockDB:         func() {},
		},
		{
			name:           "Task not found",
			taskID:         "999",
			expectedStatus: http.StatusNotFound,
			mockDB: func() {
				mock.ExpectExec(`DELETE FROM tasks WHERE id = \$1`).
					WithArgs(999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockDB()

			req := httptest.NewRequest("DELETE", "/api/v1/tasks/"+tt.taskID, nil)
			w := httptest.NewRecorder()

			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.taskID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			handler.DeleteTask(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTaskHandler_ListTasks(t *testing.T) {
	handler, mock := setupTestHandler(t)

	mock.ExpectQuery(`SELECT id, title, description, status, created_at, updated_at FROM tasks ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "created_at", "updated_at"}).
			AddRow(1, "Task 1", "Description 1", "pending", time.Now(), time.Now()).
			AddRow(2, "Task 2", "Description 2", "completed", time.Now(), time.Now()))

	req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	w := httptest.NewRecorder()

	handler.ListTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.Task
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}
