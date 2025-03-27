package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/queuet/internal/models"
	"github.com/redis/go-redis/v9"
)

// RedisClient is an interface for Redis operations
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type TaskHandler struct {
	db    *sql.DB
	cache RedisClient
}

func NewTaskHandler(db *sql.DB, cache RedisClient) *TaskHandler {
	return &TaskHandler{
		db:    db,
		cache: cache,
	}
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Insert task into database
	query := `
		INSERT INTO tasks (title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id`

	var taskID int64
	now := time.Now()
	err := h.db.QueryRow(
		query,
		req.Title,
		req.Description,
		"pending", // Default status
		now,
	).Scan(&taskID)

	if err != nil {
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Return the created task ID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": taskID})
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Try to get task from cache first
	cacheKey := fmt.Sprintf("task:%d", taskID)
	cachedTask, err := h.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedTask))
		return
	}

	// Cache miss, get from database
	query := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks
		WHERE id = $1`

	var task models.Task
	err = h.db.QueryRow(query, taskID).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to get task", http.StatusInternalServerError)
		return
	}

	// Cache the task for future requests
	taskJSON, _ := json.Marshal(task)
	h.cache.Set(ctx, cacheKey, taskJSON, time.Hour)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate status
	if req.Status != "" && req.Status != "pending" && req.Status != "in_progress" && req.Status != "completed" {
		http.Error(w, "Invalid status value", http.StatusBadRequest)
		return
	}

	// Update task in database
	query := `
		UPDATE tasks
		SET title = COALESCE($1, title),
			description = COALESCE($2, description),
			status = COALESCE($3, status),
			updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, status, created_at, updated_at`

	var task models.Task
	now := time.Now()
	err = h.db.QueryRow(
		query,
		sql.NullString{String: req.Title, Valid: req.Title != ""},
		sql.NullString{String: req.Description, Valid: req.Description != ""},
		sql.NullString{String: req.Status, Valid: req.Status != ""},
		now,
		taskID,
	).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	// Update cache
	ctx := r.Context()
	cacheKey := fmt.Sprintf("task:%d", taskID)
	taskJSON, _ := json.Marshal(task)
	h.cache.Set(ctx, cacheKey, taskJSON, time.Hour)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Delete task from database
	query := `DELETE FROM tasks WHERE id = $1`
	result, err := h.db.Exec(query, taskID)
	if err != nil {
		http.Error(w, "Failed to delete task", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to get rows affected", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Delete from cache
	ctx := r.Context()
	cacheKey := fmt.Sprintf("task:%d", taskID)
	h.cache.Del(ctx, cacheKey)

	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// Get pagination parameters
	page := 1
	pageSize := 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			pageSize = s
		}
	}

	offset := (page - 1) * pageSize

	// Get tasks from database with pagination
	query := `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := h.db.Query(query, pageSize, offset)
	if err != nil {
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			http.Error(w, "Failed to scan task", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error iterating tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
