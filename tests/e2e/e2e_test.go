package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/api"
	"github.com/Nero7991/devlm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2EWorkflow(t *testing.T) {
	cfg, err := config.Load()
	require.NoError(t, err)

	client := api.NewClient(cfg.APIEndpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	t.Run("Requirement Analysis", testRequirementAnalysis(ctx, client))
	t.Run("Code Generation", testCodeGeneration(ctx, client))
	t.Run("Code Execution", testCodeExecution(ctx, client))
	t.Run("Web Search", testWebSearch(ctx, client))
	t.Run("Code Update", testCodeUpdate(ctx, client))
	t.Run("Final Execution", testFinalExecution(ctx, client))
	t.Run("Server Functionality", testServerFunctionality)
}

func testRequirementAnalysis(ctx context.Context, client *api.Client) func(*testing.T) {
	return func(t *testing.T) {
		requirements := `Create a robust Go web server that responds with "Hello, World!" on the root path.
		Implement a /health endpoint for health checks, proper error handling for non-existent routes,
		and graceful shutdown. Use best practices for performance and security.`
		err := ioutil.WriteFile("dev.txt", []byte(requirements), 0644)
		require.NoError(t, err)
		defer os.Remove("dev.txt")

		analysisResult, err := client.AnalyzeRequirements(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, analysisResult)
		assert.Contains(t, analysisResult, "web server")
		assert.Contains(t, analysisResult, "Hello, World!")
		assert.Contains(t, analysisResult, "root path")
		assert.Contains(t, analysisResult, "Go")
		assert.Contains(t, analysisResult, "/health")
		assert.Contains(t, analysisResult, "error handling")
		assert.Contains(t, analysisResult, "graceful shutdown")
		assert.Contains(t, analysisResult, "best practices")
		assert.Contains(t, analysisResult, "performance")
		assert.Contains(t, analysisResult, "security")
	}
}

func testCodeGeneration(ctx context.Context, client *api.Client) func(*testing.T) {
	return func(t *testing.T) {
		generatedCode, err := client.GenerateCode(ctx)
		require.NoError(t, err)
		assert.Contains(t, generatedCode, "package main")
		assert.Contains(t, generatedCode, "import (")
		assert.Contains(t, generatedCode, "\"net/http\"")
		assert.Contains(t, generatedCode, "\"context\"")
		assert.Contains(t, generatedCode, "\"os\"")
		assert.Contains(t, generatedCode, "\"os/signal\"")
		assert.Contains(t, generatedCode, "\"syscall\"")
		assert.Contains(t, generatedCode, "\"time\"")
		assert.Contains(t, generatedCode, "mux := http.NewServeMux()")
		assert.Contains(t, generatedCode, "mux.HandleFunc(\"/\"")
		assert.Contains(t, generatedCode, "fmt.Fprintf(w, \"Hello, World!\")")
		assert.Contains(t, generatedCode, "mux.HandleFunc(\"/health\"")
		assert.Contains(t, generatedCode, "server := &http.Server{")
		assert.Contains(t, generatedCode, "gracefulShutdown")
	}
}

func testCodeExecution(ctx context.Context, client *api.Client) func(*testing.T) {
	return func(t *testing.T) {
		generatedCode, err := client.GenerateCode(ctx)
		require.NoError(t, err)

		executionResult, err := client.ExecuteCode(ctx, generatedCode)
		require.NoError(t, err)
		assert.True(t, executionResult.Success)
		assert.Contains(t, executionResult.Output, "Server is running")
		assert.NotContains(t, executionResult.Output, "error")

		invalidCode := `
package main

import "fmt"

func main() {
	fmt.Println("This should compile but fail at runtime")
	panic("Simulated runtime error")
}
`
		failedExecution, err := client.ExecuteCode(ctx, invalidCode)
		require.NoError(t, err)
		assert.False(t, failedExecution.Success)
		assert.Contains(t, failedExecution.Output, "panic")

		syntaxErrorCode := `
package main

func main() {
	fmt.Println("This has a syntax error"
}
`
		syntaxErrorExecution, err := client.ExecuteCode(ctx, syntaxErrorCode)
		require.NoError(t, err)
		assert.False(t, syntaxErrorExecution.Success)
		assert.Contains(t, syntaxErrorExecution.Output, "syntax error")
	}
}

func testWebSearch(ctx context.Context, client *api.Client) func(*testing.T) {
	return func(t *testing.T) {
		searchResults, err := client.PerformWebSearch(ctx, "Go web server best practices security")
		require.NoError(t, err)
		assert.NotEmpty(t, searchResults)
		assert.Contains(t, searchResults, "Go")
		assert.Contains(t, searchResults, "web server")
		assert.Contains(t, searchResults, "best practices")
		assert.Contains(t, searchResults, "security")
		assert.Contains(t, searchResults, "HTTPS")

		searchResults, err = client.PerformWebSearch(ctx, "Go graceful shutdown patterns")
		require.NoError(t, err)
		assert.NotEmpty(t, searchResults)
		assert.Contains(t, searchResults, "graceful shutdown")
		assert.Contains(t, searchResults, "context")
		assert.Contains(t, searchResults, "signal")

		emptyResults, err := client.PerformWebSearch(ctx, "")
		require.NoError(t, err)
		assert.Empty(t, emptyResults)

		irrelevantResults, err := client.PerformWebSearch(ctx, "unrelated topic XYZ123")
		require.NoError(t, err)
		assert.NotContains(t, irrelevantResults, "Go")
		assert.NotContains(t, irrelevantResults, "web server")
	}
}

func testCodeUpdate(ctx context.Context, client *api.Client) func(*testing.T) {
	return func(t *testing.T) {
		generatedCode, err := client.GenerateCode(ctx)
		require.NoError(t, err)

		searchResults, err := client.PerformWebSearch(ctx, "Go web server best practices security")
		require.NoError(t, err)

		updatedCode, err := client.UpdateCode(ctx, generatedCode, searchResults)
		require.NoError(t, err)
		assert.Contains(t, updatedCode, "http.ListenAndServeTLS")
		assert.Contains(t, updatedCode, "log.Fatal")
		assert.Contains(t, updatedCode, "http.ServeMux")
		assert.Contains(t, updatedCode, "mux := http.NewServeMux()")
		assert.Contains(t, updatedCode, "defer")
		assert.Contains(t, updatedCode, "context")
		assert.Contains(t, updatedCode, "os.Signal")
		assert.Contains(t, updatedCode, "time.Duration")
		assert.Contains(t, updatedCode, "http.TimeoutHandler")
		assert.Contains(t, updatedCode, "MaxHeaderBytes")
		assert.Contains(t, updatedCode, "TLSConfig")
	}
}

func testFinalExecution(ctx context.Context, client *api.Client) func(*testing.T) {
	return func(t *testing.T) {
		updatedCode, err := client.UpdateCode(ctx, "", "")
		require.NoError(t, err)

		finalExecutionResult, err := client.ExecuteCode(ctx, updatedCode)
		require.NoError(t, err)
		assert.True(t, finalExecutionResult.Success)
		assert.Contains(t, finalExecutionResult.Output, "Server is running")
		assert.NotContains(t, finalExecutionResult.Output, "error")
		assert.Contains(t, finalExecutionResult.Output, "Listening on :8080")
		assert.Contains(t, finalExecutionResult.Output, "Shutting down server...")
		assert.Contains(t, finalExecutionResult.Output, "Server gracefully stopped")
	}
}

func testServerFunctionality(t *testing.T) {
	time.Sleep(2 * time.Second)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://localhost:8080")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Hello, World!", string(body))

	resp, err = client.Get("http://localhost:8080/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = client.Get("http://localhost:8080/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp, err = client.Post("http://localhost:8080", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	req, err := http.NewRequest("GET", "http://localhost:8080", nil)
	require.NoError(t, err)
	req.Header.Set("X-XSS-Protection", "1; mode=block")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
}

func TestConcurrentRequests(t *testing.T) {
	concurrentRequests := 1000
	results := make(chan bool, concurrentRequests)
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get("http://localhost:8080")
			if err != nil || resp.StatusCode != http.StatusOK {
				results <- false
				return
			}
			defer resp.Body.Close()
			results <- true
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	for result := range results {
		if result {
			successCount++
		}
	}

	assert.Equal(t, concurrentRequests, successCount, "All concurrent requests should succeed")
}

func TestServerStress(t *testing.T) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	requestCount := 5000
	var wg sync.WaitGroup
	errors := make(chan error, requestCount)

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get("http://localhost:8080")
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}
		}()
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Logf("Error during stress test: %v", err)
		errorCount++
	}

	assert.LessOrEqual(t, errorCount, requestCount/200, "Error rate should be less than 0.5%")
}

func TestLargePayload(t *testing.T) {
	largePayload := make([]byte, 50*1024*1024) // 50 MB payload
	req, err := http.NewRequest("POST", "http://localhost:8080/large", bytes.NewReader(largePayload))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(largePayload))

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	duration := time.Since(start)
	assert.LessOrEqual(t, duration, 5*time.Second, "Large payload request should complete within 5 seconds")

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Received large payload")

	tooLargePayload := make([]byte, 150*1024*1024) // 150 MB payload
	req, err = http.NewRequest("POST", "http://localhost:8080/large", bytes.NewReader(tooLargePayload))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(tooLargePayload))

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}