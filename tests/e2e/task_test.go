package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/queuet/internal/cache"
	"github.com/queuet/internal/database"
	"github.com/queuet/internal/handlers"
	"github.com/queuet/internal/models"
	"github.com/queuet/internal/routes"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

type TaskE2ETestSuite struct {
	suite.Suite
	db          *sql.DB
	cache       *redis.Client
	taskHandler *handlers.TaskHandler
	server      *http.Server
}

func (s *TaskE2ETestSuite) SetupSuite() {
	// Initialize database connection
	dbConfig := database.NewConfig()
	dbConfig.Host = os.Getenv("POSTGRES_HOST")
	dbConfig.Port = os.Getenv("POSTGRES_PORT")
	dbConfig.User = os.Getenv("POSTGRES_USER")
	dbConfig.Password = os.Getenv("POSTGRES_PASSWORD")
	dbConfig.DBName = os.Getenv("POSTGRES_DB")

	db, err := database.Connect(dbConfig)
	s.Require().NoError(err)
	s.db = db

	// Initialize Redis connection
	redisConfig := cache.NewRedisConfig()
	redisConfig.Host = os.Getenv("REDIS_HOST")
	port, _ := strconv.Atoi(os.Getenv("REDIS_PORT"))
	redisConfig.Port = port

	redisClient, err := cache.NewRedisClient(redisConfig)
	s.Require().NoError(err)
	s.cache = redisClient

	// Initialize task handler
	s.taskHandler = handlers.NewTaskHandler(s.db, s.cache)

	// Start the server
	r := chi.NewRouter()
	routes.SetupRoutes(r, s.taskHandler)

	s.server = &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.T().Errorf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
}

func (s *TaskE2ETestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
	if s.cache != nil {
		s.cache.Close()
	}
}

func (s *TaskE2ETestSuite) TestCompleteTaskFlow() {
	// Create a task
	task := models.Task{
		Title:       "Test Task",
		Description: "Test Description",
	}

	taskJSON, err := json.Marshal(task)
	s.Require().NoError(err)

	resp, err := http.Post("http://localhost:8080/api/v1/tasks", "application/json", bytes.NewBuffer(taskJSON))
	s.Require().NoError(err)
	s.Equal(http.StatusCreated, resp.StatusCode)

	var createdTask models.Task
	err = json.NewDecoder(resp.Body).Decode(&createdTask)
	s.Require().NoError(err)
	resp.Body.Close()

	// Get the task
	resp, err = http.Get(fmt.Sprintf("http://localhost:8080/api/v1/tasks/%d", createdTask.ID))
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var retrievedTask models.Task
	err = json.NewDecoder(resp.Body).Decode(&retrievedTask)
	s.Require().NoError(err)
	resp.Body.Close()

	s.Equal(createdTask.ID, retrievedTask.ID)
	s.Equal(task.Title, retrievedTask.Title)
	s.Equal(task.Description, retrievedTask.Description)

	// Update the task
	retrievedTask.Status = "completed"
	taskJSON, err = json.Marshal(retrievedTask)
	s.Require().NoError(err)

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://localhost:8080/api/v1/tasks/%d", retrievedTask.ID), bytes.NewBuffer(taskJSON))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err = client.Do(req)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var updatedTask models.Task
	err = json.NewDecoder(resp.Body).Decode(&updatedTask)
	s.Require().NoError(err)
	resp.Body.Close()

	s.Equal(retrievedTask.ID, updatedTask.ID)
	s.Equal("completed", updatedTask.Status)

	// Delete the task
	req, err = http.NewRequest("DELETE", fmt.Sprintf("http://localhost:8080/api/v1/tasks/%d", updatedTask.ID), nil)
	s.Require().NoError(err)

	resp, err = client.Do(req)
	s.Require().NoError(err)
	s.Equal(http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify task is deleted
	resp, err = http.Get(fmt.Sprintf("http://localhost:8080/api/v1/tasks/%d", updatedTask.ID))
	s.Require().NoError(err)
	s.Equal(http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func (s *TaskE2ETestSuite) TestTaskValidation() {
	// Test empty title
	task := models.CreateTaskRequest{
		Title:       "",
		Description: "Test Description",
	}

	taskJSON, err := json.Marshal(task)
	s.Require().NoError(err)

	resp, err := http.Post("http://localhost:8080/api/v1/tasks", "application/json", bytes.NewBuffer(taskJSON))
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Test invalid status
	task = models.CreateTaskRequest{
		Title:       "Test Task",
		Description: "Test Description",
	}

	taskJSON, err = json.Marshal(task)
	s.Require().NoError(err)

	resp, err = http.Post("http://localhost:8080/api/v1/tasks", "application/json", bytes.NewBuffer(taskJSON))
	s.Require().NoError(err)
	s.Equal(http.StatusCreated, resp.StatusCode)

	// Get the task ID from the response
	var createResp struct {
		ID int64 `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	s.Require().NoError(err)
	resp.Body.Close()

	// Try to update with invalid status
	updateReq := models.UpdateTaskRequest{
		Status: "invalid",
	}

	taskJSON, err = json.Marshal(updateReq)
	s.Require().NoError(err)

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://localhost:8080/api/v1/tasks/%d", createResp.ID), bytes.NewBuffer(taskJSON))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err = client.Do(req)
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestTaskE2E(t *testing.T) {
	suite.Run(t, new(TaskE2ETestSuite))
}
