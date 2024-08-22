package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/api"
	"github.com/Nero7991/devlm/internal/cache"
	"github.com/Nero7991/devlm/internal/database"
	"github.com/Nero7991/devlm/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIIntegration(t *testing.T) {
	ctx := context.Background()
	db, err := database.NewPostgresDB(ctx, "postgres://user:password@localhost:5432/devlm_test?sslmode=disable")
	require.NoError(t, err)
	defer db.Close()

	redisClient, err := cache.NewRedisClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	llmService := services.NewLLMService("http://localhost:8000")
	workerService := services.NewWorkerService()

	router := api.SetupRouter(db, redisClient, llmService, workerService)

	t.Run("TestCreateProject", testCreateProject(router))
	t.Run("TestGetProject", testGetProject(router))
	t.Run("TestUpdateProject", testUpdateProject(router))
	t.Run("TestDeleteProject", testDeleteProject(router))
	t.Run("TestGenerateCode", testGenerateCode(router))
	t.Run("TestExecuteCode", testExecuteCode(router))

	t.Cleanup(func() {
		_, err := db.Exec(ctx, "DELETE FROM projects")
		require.NoError(t, err)
		_, err = db.Exec(ctx, "DELETE FROM code_generations")
		require.NoError(t, err)
		_, err = db.Exec(ctx, "DELETE FROM code_executions")
		require.NoError(t, err)
	})
}

func testCreateProject(router http.Handler) func(*testing.T) {
	return func(t *testing.T) {
		projectData := map[string]interface{}{
			"name":        "Test Project",
			"description": "This is a test project",
		}
		jsonData, _ := json.Marshal(projectData)

		req, _ := http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.NotNil(t, response["id"])
		assert.Equal(t, projectData["name"], response["name"])
		assert.Equal(t, projectData["description"], response["description"])

		// Test invalid input
		invalidProjectData := map[string]interface{}{
			"name": "",
		}
		jsonData, _ = json.Marshal(invalidProjectData)
		req, _ = http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test duplicate project name
		req, _ = http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)

		// Test project name with special characters
		specialCharProjectData := map[string]interface{}{
			"name":        "Test Project @#$%",
			"description": "This is a test project with special characters",
		}
		jsonData, _ = json.Marshal(specialCharProjectData)
		req, _ = http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test project with very long name and description
		longProjectData := map[string]interface{}{
			"name":        string(make([]byte, 256)),
			"description": string(make([]byte, 1001)),
		}
		jsonData, _ = json.Marshal(longProjectData)
		req, _ = http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test concurrent project creation
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				concurrentProjectData := map[string]interface{}{
					"name":        fmt.Sprintf("Concurrent Project %d", i),
					"description": "This is a concurrent test project",
				}
				jsonData, _ := json.Marshal(concurrentProjectData)
				req, _ := http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusConflict)
			}(i)
		}
		wg.Wait()
	}
}

func testGetProject(router http.Handler) func(*testing.T) {
	return func(t *testing.T) {
		projectData := map[string]interface{}{
			"name":        "Test Project for Get",
			"description": "This is a test project for get operation",
		}
		jsonData, _ := json.Marshal(projectData)

		createReq, _ := http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		createReq.Header.Set("Content-Type", "application/json")

		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createReq)

		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		projectID := createResponse["id"].(float64)

		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/projects/%.0f", projectID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, projectID, response["id"])
		assert.Equal(t, projectData["name"], response["name"])
		assert.Equal(t, projectData["description"], response["description"])

		// Test non-existent project
		nonExistentReq, _ := http.NewRequest("GET", "/api/projects/999999", nil)
		nonExistentW := httptest.NewRecorder()
		router.ServeHTTP(nonExistentW, nonExistentReq)
		assert.Equal(t, http.StatusNotFound, nonExistentW.Code)

		// Test invalid ID
		invalidReq, _ := http.NewRequest("GET", "/api/projects/invalid", nil)
		invalidW := httptest.NewRecorder()
		router.ServeHTTP(invalidW, invalidReq)
		assert.Equal(t, http.StatusBadRequest, invalidW.Code)

		// Test with query parameters for pagination
		paginatedReq, _ := http.NewRequest("GET", "/api/projects?page=1&limit=10", nil)
		paginatedW := httptest.NewRecorder()
		router.ServeHTTP(paginatedW, paginatedReq)
		assert.Equal(t, http.StatusOK, paginatedW.Code)

		var paginatedResponse map[string]interface{}
		json.Unmarshal(paginatedW.Body.Bytes(), &paginatedResponse)
		assert.NotNil(t, paginatedResponse["data"])
		assert.NotNil(t, paginatedResponse["total"])
		assert.NotNil(t, paginatedResponse["page"])
		assert.NotNil(t, paginatedResponse["limit"])

		// Test filtering and sorting
		filterReq, _ := http.NewRequest("GET", "/api/projects?sort=name&order=desc&filter=test", nil)
		filterW := httptest.NewRecorder()
		router.ServeHTTP(filterW, filterReq)
		assert.Equal(t, http.StatusOK, filterW.Code)

		var filterResponse map[string]interface{}
		json.Unmarshal(filterW.Body.Bytes(), &filterResponse)
		assert.NotNil(t, filterResponse["data"])
		assert.NotEmpty(t, filterResponse["data"])
	}
}

func testUpdateProject(router http.Handler) func(*testing.T) {
	return func(t *testing.T) {
		projectData := map[string]interface{}{
			"name":        "Test Project for Update",
			"description": "This is a test project for update operation",
		}
		jsonData, _ := json.Marshal(projectData)

		createReq, _ := http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		createReq.Header.Set("Content-Type", "application/json")

		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createReq)

		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		projectID := createResponse["id"].(float64)

		updateData := map[string]interface{}{
			"name":        "Updated Test Project",
			"description": "This is an updated test project",
		}
		jsonData, _ = json.Marshal(updateData)

		req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/projects/%.0f", projectID), bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, projectID, response["id"])
		assert.Equal(t, updateData["name"], response["name"])
		assert.Equal(t, updateData["description"], response["description"])

		// Test partial update
		partialUpdateData := map[string]interface{}{
			"name": "Partially Updated Project",
		}
		jsonData, _ = json.Marshal(partialUpdateData)
		partialReq, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/projects/%.0f", projectID), bytes.NewBuffer(jsonData))
		partialReq.Header.Set("Content-Type", "application/json")
		partialW := httptest.NewRecorder()
		router.ServeHTTP(partialW, partialReq)
		assert.Equal(t, http.StatusOK, partialW.Code)

		var partialResponse map[string]interface{}
		json.Unmarshal(partialW.Body.Bytes(), &partialResponse)
		assert.Equal(t, partialUpdateData["name"], partialResponse["name"])
		assert.Equal(t, updateData["description"], partialResponse["description"])

		// Test invalid update data
		invalidUpdateData := map[string]interface{}{
			"name": "",
		}
		jsonData, _ = json.Marshal(invalidUpdateData)
		invalidReq, _ := http.NewRequest("PUT", fmt.Sprintf("/api/projects/%.0f", projectID), bytes.NewBuffer(jsonData))
		invalidReq.Header.Set("Content-Type", "application/json")
		invalidW := httptest.NewRecorder()
		router.ServeHTTP(invalidW, invalidReq)
		assert.Equal(t, http.StatusBadRequest, invalidW.Code)

		// Test concurrent updates
		var wg sync.WaitGroup
		updateResults := make(chan int, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				concurrentUpdateData := map[string]interface{}{
					"name": fmt.Sprintf("Concurrent Update %d", i),
				}
				jsonData, _ := json.Marshal(concurrentUpdateData)
				concurrentReq, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/projects/%.0f", projectID), bytes.NewBuffer(jsonData))
				concurrentReq.Header.Set("Content-Type", "application/json")
				concurrentW := httptest.NewRecorder()
				router.ServeHTTP(concurrentW, concurrentReq)
				updateResults <- concurrentW.Code
			}(i)
		}

		wg.Wait()
		close(updateResults)

		successCount := 0
		conflictCount := 0
		for code := range updateResults {
			if code == http.StatusOK {
				successCount++
			} else if code == http.StatusConflict {
				conflictCount++
			}
		}

		assert.True(t, successCount > 0)
		assert.True(t, conflictCount > 0)
		assert.Equal(t, 10, successCount+conflictCount)
	}
}

func testDeleteProject(router http.Handler) func(*testing.T) {
	return func(t *testing.T) {
		projectData := map[string]interface{}{
			"name":        "Test Project for Delete",
			"description": "This is a test project for delete operation",
		}
		jsonData, _ := json.Marshal(projectData)

		createReq, _ := http.NewRequest("POST", "/api/projects", bytes.NewBuffer(jsonData))
		createReq.Header.Set("Content-Type", "application/json")

		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createReq)

		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		projectID := createResponse["id"].(float64)

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/projects/%.0f", projectID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		getReq, _ := http.NewRequest("GET", fmt.Sprintf("/api/projects/%.0f", projectID), nil)
		getW := httptest.NewRecorder()
		router.ServeHTTP(getW, getReq