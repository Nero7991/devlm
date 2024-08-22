package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/worker"
	"github.com/Nero7991/devlm/pkg/cache"
	"github.com/Nero7991/devlm/pkg/database"
	"github.com/Nero7991/devlm/pkg/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerIntegration(t *testing.T) {
	db, err := database.NewConnection(os.Getenv("TEST_DB_URL"))
	require.NoError(t, err)
	defer db.Close()

	redisCache, err := cache.NewRedisCache(os.Getenv("TEST_REDIS_URL"))
	require.NoError(t, err)
	defer redisCache.Close()

	llmService := llm.NewService(os.Getenv("TEST_LLM_SERVICE_URL"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w := worker.NewWorker(db, redisCache, llmService)

	t.Run("TestProcessTask", testProcessTask(ctx, w))
	t.Run("TestHandleCodeGeneration", testHandleCodeGeneration(ctx, w))
	t.Run("TestHandleCodeExecution", testHandleCodeExecution(ctx, w))
	t.Run("TestHandleFileOperation", testHandleFileOperation(ctx, w))
	t.Run("TestHandleWebSearch", testHandleWebSearch(ctx, w))
}

func testProcessTask(ctx context.Context, w *worker.Worker) func(*testing.T) {
	return func(t *testing.T) {
		task := &worker.Task{
			ID:          "task-1",
			Type:        worker.TaskTypeCodeGeneration,
			Description: "Generate a simple Go function",
			Status:      worker.TaskStatusPending,
		}

		result, err := w.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, worker.TaskStatusCompleted, result.Status)
		assert.NotEmpty(t, result.Result)

		invalidTask := &worker.Task{
			ID:          "invalid-task",
			Type:        "InvalidTaskType",
			Description: "This task should fail",
			Status:      worker.TaskStatusPending,
		}

		_, err = w.ProcessTask(ctx, invalidTask)
		assert.Error(t, err)

		taskChan := make(chan *worker.Task, 5)
		resultChan := make(chan *worker.TaskResult, 5)
		errChan := make(chan error, 5)

		for i := 0; i < 5; i++ {
			go func() {
				task := <-taskChan
				result, err := w.ProcessTask(ctx, task)
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}()
		}

		for i := 0; i < 5; i++ {
			taskChan <- &worker.Task{
				ID:          fmt.Sprintf("concurrent-task-%d", i),
				Type:        worker.TaskTypeCodeGeneration,
				Description: fmt.Sprintf("Generate concurrent function %d", i),
				Status:      worker.TaskStatusPending,
			}
		}

		for i := 0; i < 5; i++ {
			select {
			case result := <-resultChan:
				assert.Equal(t, worker.TaskStatusCompleted, result.Status)
				assert.NotEmpty(t, result.Result)
			case err := <-errChan:
				t.Errorf("Error processing concurrent task: %v", err)
			case <-time.After(5 * time.Second):
				t.Error("Timeout waiting for concurrent task result")
			}
		}

		longRunningTask := &worker.Task{
			ID:          "long-running-task",
			Type:        worker.TaskTypeCodeExecution,
			Description: "Long-running task that should be cancelled",
			Status:      worker.TaskStatusPending,
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		_, err = w.ProcessTask(ctxWithTimeout, longRunningTask)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")

		highPriorityTask := &worker.Task{
			ID:          "high-priority-task",
			Type:        worker.TaskTypeCodeGeneration,
			Description: "High priority task",
			Status:      worker.TaskStatusPending,
			Priority:    worker.TaskPriorityHigh,
		}

		lowPriorityTask := &worker.Task{
			ID:          "low-priority-task",
			Type:        worker.TaskTypeCodeGeneration,
			Description: "Low priority task",
			Status:      worker.TaskStatusPending,
			Priority:    worker.TaskPriorityLow,
		}

		go w.ProcessTask(ctx, lowPriorityTask)
		time.Sleep(10 * time.Millisecond)

		highPriorityResult, err := w.ProcessTask(ctx, highPriorityTask)
		assert.NoError(t, err)
		assert.NotNil(t, highPriorityResult)
		assert.Equal(t, worker.TaskStatusCompleted, highPriorityResult.Status)
	}
}

func testHandleCodeGeneration(ctx context.Context, w *worker.Worker) func(*testing.T) {
	return func(t *testing.T) {
		request := &worker.CodeGenerationRequest{
			Language:    "go",
			Description: "Generate a function that calculates the factorial of a number",
		}

		result, err := w.HandleCodeGeneration(ctx, request)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.Code, "func factorial")
		assert.NotEmpty(t, result.Explanation)

		pythonRequest := &worker.CodeGenerationRequest{
			Language:    "python",
			Description: "Generate a function that calculates the Fibonacci sequence",
		}

		pythonResult, err := w.HandleCodeGeneration(ctx, pythonRequest)
		assert.NoError(t, err)
		assert.NotNil(t, pythonResult)
		assert.Contains(t, pythonResult.Code, "def fibonacci")
		assert.NotEmpty(t, pythonResult.Explanation)

		invalidRequest := &worker.CodeGenerationRequest{
			Language:    "invalid",
			Description: "This should fail",
		}

		_, err = w.HandleCodeGeneration(ctx, invalidRequest)
		assert.Error(t, err)

		constrainedRequest := &worker.CodeGenerationRequest{
			Language:    "go",
			Description: "Generate a function that sorts an array of integers",
			Constraints: []string{"Use quicksort algorithm", "Handle empty input gracefully"},
		}

		constrainedResult, err := w.HandleCodeGeneration(ctx, constrainedRequest)
		assert.NoError(t, err)
		assert.NotNil(t, constrainedResult)
		assert.Contains(t, constrainedResult.Code, "func quicksort")
		assert.Contains(t, constrainedResult.Code, "if len(")
		assert.NotEmpty(t, constrainedResult.Explanation)

		complexRequest := &worker.CodeGenerationRequest{
			Language:    "go",
			Description: "Generate a concurrent web scraper that fetches and processes data from multiple URLs",
			Constraints: []string{"Use goroutines", "Implement rate limiting", "Handle errors gracefully"},
		}

		complexResult, err := w.HandleCodeGeneration(ctx, complexRequest)
		assert.NoError(t, err)
		assert.NotNil(t, complexResult)
		assert.Contains(t, complexResult.Code, "go func()")
		assert.Contains(t, complexResult.Code, "rate.NewLimiter")
		assert.Contains(t, complexResult.Code, "if err != nil")
		assert.NotEmpty(t, complexResult.Explanation)

		optimizationRequest := &worker.CodeGenerationRequest{
			Language:    "go",
			Description: "Optimize the following function for performance",
			Code: `
				func fibonacci(n int) int {
					if n <= 1 {
						return n
					}
					return fibonacci(n-1) + fibonacci(n-2)
				}
			`,
			Constraints: []string{"Improve time complexity", "Use memoization"},
		}

		optimizedResult, err := w.HandleCodeGeneration(ctx, optimizationRequest)
		assert.NoError(t, err)
		assert.NotNil(t, optimizedResult)
		assert.Contains(t, optimizedResult.Code, "map[int]int")
		assert.Contains(t, optimizedResult.Explanation, "memoization")
	}
}

func testHandleCodeExecution(ctx context.Context, w *worker.Worker) func(*testing.T) {
	return func(t *testing.T) {
		request := &worker.CodeExecutionRequest{
			Language: "go",
			Code: `
				package main
				import "fmt"
				func main() {
					fmt.Println("Hello, DevLM!")
				}
			`,
		}

		result, err := w.HandleCodeExecution(ctx, request)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.Output, "Hello, DevLM!")
		assert.Equal(t, 0, result.ExitCode)

		pythonRequest := &worker.CodeExecutionRequest{
			Language: "python",
			Code: `
				print("Hello from Python!")
			`,
		}

		pythonResult, err := w.HandleCodeExecution(ctx, pythonRequest)
		assert.NoError(t, err)
		assert.NotNil(t, pythonResult)
		assert.Contains(t, pythonResult.Output, "Hello from Python!")
		assert.Equal(t, 0, pythonResult.ExitCode)

		invalidRequest := &worker.CodeExecutionRequest{
			Language: "go",
			Code:     "This is not valid Go code",
		}

		_, err = w.HandleCodeExecution(ctx, invalidRequest)
		assert.Error(t, err)

		timeoutRequest := &worker.CodeExecutionRequest{
			Language: "go",
			Code: `
				package main
				import (
					"fmt"
					"time"
				)
				func main() {
					time.Sleep(10 * time.Second)
					fmt.Println("This should not be printed")
				}
			`,
			Timeout: 2 * time.Second,
		}

		timeoutResult, err := w.HandleCodeExecution(ctx, timeoutRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "execution timed out")
		assert.Nil(t, timeoutResult)

		resourceIntensiveRequest := &worker.CodeExecutionRequest{
			Language: "go",
			Code: `
				package main
				func main() {
					data := make([]int, 1000000000)
					for i := 0; i < len(data); i++ {
						data[i] = i
					}
				}
			`,
			MemoryLimit: 10 * 1024 * 1024, // 10 MB
		}

		_, err = w.HandleCodeExecution(ctx, resourceIntensiveRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory limit exceeded")

		dockerRequest := &worker.CodeExecutionRequest{
			Language: "python",
			Code: `
				import platform
				print(f"Python version: {platform.python_version()}")
			`,
			Environment: "docker:python:3.9",
		}

		dockerResult, err := w.HandleCodeExecution(ctx, dockerRequest)
		assert.NoError(t, err)
		assert.NotNil(t, dockerResult)
		assert.Contains(t, dockerResult.Output, "Python version: 3.9")
	}
}

func testHandleFileOperation(ctx context.Context, w *worker.Worker) func(*testing.T) {
	return func(t *testing.T) {
		request := &worker.FileOperationRequest{
			Operation: worker.FileOperationWrite,
			Path:      "/tmp/devlm_test.txt",
			Content:   "This is a test file for DevLM",
		}

		result, err := w.HandleFileOperation(ctx, request)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)

		readRequest := &worker.FileOperationRequest{
			Operation: worker.FileOperationRead,
			Path:      "/tmp/devlm_test.txt",
		}

		readResult, err := w.HandleFileOperation(ctx, readRequest)
		assert.NoError(t, err)
		assert.NotNil(t, readResult)
		assert.Equal(t, request.Content, readResult.Content)

		updateRequest := &worker.FileOperationRequest{
			Operation: worker.FileOperationWrite,
			Path:      "/tmp/devlm_test.txt",
			Content:   "Updated content",
		}

		updateResult, err := w.HandleFileOperation(ctx, updateRequest)
		assert.NoError(t, err)
		assert.NotNil(t, updateResult)
		assert.True(t, updateResult.Success)

		deleteRequest := &worker.FileOperationRequest{
			Operation: worker.FileOperationDelete,
			Path:      "/tmp/devlm_test.txt",
		}

		deleteResult, err := w.HandleFileOperation(ctx, deleteRequest)
		assert.NoError(t, err)
		assert.NotNil(t, deleteResult)
		assert.True(t, deleteResult.Success)

		_, err = w.HandleFileOperation(ctx, readRequest)
		assert.Error(t, err)

		nonExistentRequest := &worker.FileOperationRequest{
			Operation: worker.FileOperationWrite,
			Path:      "/nonexistent/directory/file.txt",
			Content:   "This should fail",
		}

		_, err = w.HandleFileOperation(ctx, nonExistentRequest)
		assert.Error(t, err)

		largeFileRequest := &worker.FileOperationRequest{
			Operation: worker.FileOperationWrite,
			Path:      "/tmp/large_file.txt",
			Content:   string(make([]byte, 100*1024*1024)), // 100 MB
		}

		_, err = w.HandleFileOperation(ctx, largeFileRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file size exceeds limit")

		restrictedRequest := &worker.FileOperationRequest{
			Operation: worker.FileOperationWrite,
			Path:      "/etc/hosts",
			Content:   "This should fail due to insufficient permissions",
		}

		_, err = w.HandleFileOperation(ctx, restrictedRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")

		concurrentWrites := 10
		var wg sync.WaitGroup
		wg.Add(concurrentWrites)

		for i := 0; i < concurrentWrites; i++ {
			go func(i int) {