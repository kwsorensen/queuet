package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/queuet/internal/cache"
	"github.com/queuet/internal/database"
	"github.com/queuet/internal/handlers"
	"github.com/queuet/internal/models"
	"github.com/queuet/internal/routes"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type E2ETestSuite struct {
	suite.Suite
	router      chi.Router
	server      *httptest.Server
	db          *sql.DB
	redisClient *redis.Client
}

func TestE2ESuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}
	suite.Run(t, new(E2ETestSuite))
}

func (s *E2ETestSuite) SetupSuite() {
	var err error

	// Initialize router
	s.router = chi.NewRouter()

	// Connect to database
	dbConfig := database.NewConfig()
	s.db, err = database.Connect(dbConfig)
	if err != nil {
		s.T().Fatalf("Failed to connect to database: %v", err)
	}

	// Connect to Redis
	redisConfig := cache.NewRedisConfig()
	s.redisClient, err = cache.NewRedisClient(redisConfig)
	if err != nil {
		s.T().Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize handler with real dependencies
	taskHandler := handlers.NewTaskHandler(s.db, s.redisClient)

	// Setup routes with the configured handler
	routes.SetupRoutes(s.router, taskHandler)

	// Create test server
	s.server = httptest.NewServer(s.router)
}

func (s *E2ETestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
}

func (s *E2ETestSuite) TestCompleteTaskFlow() {
	t := s.T()
	baseURL := s.server.URL

	// 1. Create a new task
	createPayload := models.CreateTaskRequest{
		Title:       "Test E2E Task",
		Description: "This is an E2E test task",
	}
	createBody, _ := json.Marshal(createPayload)

	createResp, err := http.Post(
		fmt.Sprintf("%s/api/v1/tasks", baseURL),
		"application/json",
		bytes.NewBuffer(createBody),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	// Get the task ID from the response
	var createResult struct {
		ID int64 `json:"id"`
	}
	err = json.NewDecoder(createResp.Body).Decode(&createResult)
	assert.NoError(t, err)
	createResp.Body.Close()

	// 2. Get the created task
	getResp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks/%d", baseURL, createResult.ID))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var getResult models.Task
	err = json.NewDecoder(getResp.Body).Decode(&getResult)
	assert.NoError(t, err)
	assert.Equal(t, createPayload.Title, getResult.Title)
	assert.Equal(t, createPayload.Description, getResult.Description)
	getResp.Body.Close()

	// 3. Update the task
	updatePayload := models.UpdateTaskRequest{
		Title:  "Updated E2E Task",
		Status: "completed",
	}
	updateBody, _ := json.Marshal(updatePayload)

	req, _ := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v1/tasks/%d", baseURL, createResult.ID),
		bytes.NewBuffer(updateBody),
	)
	req.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)
	updateResp.Body.Close()

	// 4. Verify the update
	getResp, err = http.Get(fmt.Sprintf("%s/api/v1/tasks/%d", baseURL, createResult.ID))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	err = json.NewDecoder(getResp.Body).Decode(&getResult)
	assert.NoError(t, err)
	assert.Equal(t, updatePayload.Title, getResult.Title)
	assert.Equal(t, updatePayload.Status, getResult.Status)
	getResp.Body.Close()

	// 5. List all tasks
	listResp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks", baseURL))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var listResult []models.Task
	err = json.NewDecoder(listResp.Body).Decode(&listResult)
	assert.NoError(t, err)
	assert.NotEmpty(t, listResult)
	listResp.Body.Close()

	// 6. Delete the task
	req, _ = http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/api/v1/tasks/%d", baseURL, createResult.ID),
		nil,
	)

	deleteResp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)
	deleteResp.Body.Close()

	// 7. Verify deletion
	getResp, err = http.Get(fmt.Sprintf("%s/api/v1/tasks/%d", baseURL, createResult.ID))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
	getResp.Body.Close()
}

func (s *E2ETestSuite) TestTaskValidation() {
	t := s.T()
	baseURL := s.server.URL

	// Test creating task with empty title
	createPayload := models.CreateTaskRequest{
		Title:       "",
		Description: "This should fail validation",
	}
	createBody, _ := json.Marshal(createPayload)

	createResp, err := http.Post(
		fmt.Sprintf("%s/api/v1/tasks", baseURL),
		"application/json",
		bytes.NewBuffer(createBody),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, createResp.StatusCode)
	createResp.Body.Close()

	// Test invalid task ID
	getResp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks/invalid", baseURL))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, getResp.StatusCode)
	getResp.Body.Close()
}
