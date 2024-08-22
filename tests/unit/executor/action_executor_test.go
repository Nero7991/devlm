package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Nero7991/devlm/internal/executor"
	"github.com/Nero7991/devlm/internal/models"
)

type MockFileOperations struct {
	mock.Mock
}

func (m *MockFileOperations) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileOperations) WriteFile(path string, data []byte, perm uint32) error {
	args := m.Called(path, data, perm)
	return args.Error(0)
}

type MockWebSearch struct {
	mock.Mock
}

func (m *MockWebSearch) Search(query string) ([]string, error) {
	args := m.Called(query)
	return args.Get(0).([]string), args.Error(1)
}

func TestActionExecutor_Execute(t *testing.T) {
	mockFileOps := new(MockFileOperations)
	mockWebSearch := new(MockWebSearch)

	actionExecutor := executor.NewActionExecutor(mockFileOps, mockWebSearch)

	t.Run("ExecuteReadFile", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": "/test/file.txt",
			},
		}

		expectedContent := []byte("test content")
		mockFileOps.On("ReadFile", "/test/file.txt").Return(expectedContent, nil)

		result, err := actionExecutor.Execute(ctx, action)

		assert.NoError(t, err)
		assert.Equal(t, expectedContent, result)
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteReadFileNotFound", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": "/test/nonexistent.txt",
			},
		}

		mockFileOps.On("ReadFile", "/test/nonexistent.txt").Return([]byte{}, errors.New("file not found"))

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "file not found")
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWriteFile", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WriteFile",
			Params: map[string]interface{}{
				"path": "/test/file.txt",
				"data": []byte("test content"),
				"perm": uint32(0644),
			},
		}

		mockFileOps.On("WriteFile", "/test/file.txt", []byte("test content"), uint32(0644)).Return(nil)

		result, err := actionExecutor.Execute(ctx, action)

		assert.NoError(t, err)
		assert.Nil(t, result)
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWriteFilePermissionDenied", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WriteFile",
			Params: map[string]interface{}{
				"path": "/test/readonly.txt",
				"data": []byte("test content"),
				"perm": uint32(0644),
			},
		}

		mockFileOps.On("WriteFile", "/test/readonly.txt", []byte("test content"), uint32(0644)).Return(errors.New("permission denied"))

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "permission denied")
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWebSearch", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WebSearch",
			Params: map[string]interface{}{
				"query": "test query",
			},
		}

		expectedResults := []string{"result1", "result2"}
		mockWebSearch.On("Search", "test query").Return(expectedResults, nil)

		result, err := actionExecutor.Execute(ctx, action)

		assert.NoError(t, err)
		assert.Equal(t, expectedResults, result)
		mockWebSearch.AssertExpectations(t)
	})

	t.Run("ExecuteWebSearchNetworkFailure", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WebSearch",
			Params: map[string]interface{}{
				"query": "test query",
			},
		}

		mockWebSearch.On("Search", "test query").Return([]string{}, errors.New("network failure"))

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "network failure")
		mockWebSearch.AssertExpectations(t)
	})

	t.Run("ExecuteInvalidAction", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "InvalidAction",
		}

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unsupported action type")
	})

	t.Run("ExecuteMissingParams", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type:   "ReadFile",
			Params: map[string]interface{}{},
		}

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "missing required parameter")
	})

	t.Run("ExecuteReadFileLargeFile", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": "/test/largefile.txt",
			},
		}

		largeContent := make([]byte, 1024*1024*10) // 10MB file
		mockFileOps.On("ReadFile", "/test/largefile.txt").Return(largeContent, nil)

		result, err := actionExecutor.Execute(ctx, action)

		assert.NoError(t, err)
		assert.Equal(t, largeContent, result)
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWriteFileToNonExistentDirectory", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WriteFile",
			Params: map[string]interface{}{
				"path": "/nonexistent/directory/file.txt",
				"data": []byte("test content"),
				"perm": uint32(0644),
			},
		}

		mockFileOps.On("WriteFile", "/nonexistent/directory/file.txt", []byte("test content"), uint32(0644)).Return(errors.New("no such file or directory"))

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no such file or directory")
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWebSearchEmptyQuery", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WebSearch",
			Params: map[string]interface{}{
				"query": "",
			},
		}

		mockWebSearch.On("Search", "").Return([]string{}, errors.New("empty query"))

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "empty query")
		mockWebSearch.AssertExpectations(t)
	})

	t.Run("ExecuteActionWithInvalidParamType", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": 123, // Invalid type for path
			},
		}

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid parameter type")
	})

	t.Run("ExecuteReadFileWithTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": "/test/slow_file.txt",
			},
		}

		mockFileOps.On("ReadFile", "/test/slow_file.txt").After(time.Millisecond * 10).Return([]byte("content"), nil)

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "context deadline exceeded")
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWebSearchWithLongQuery", func(t *testing.T) {
		ctx := context.Background()
		longQuery := string(make([]byte, 1000))
		action := models.Action{
			Type: "WebSearch",
			Params: map[string]interface{}{
				"query": longQuery,
			},
		}

		mockWebSearch.On("Search", longQuery).Return([]string{}, errors.New("query too long"))

		result, err := actionExecutor.Execute(ctx, action)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query too long")
		mockWebSearch.AssertExpectations(t)
	})

	t.Run("ExecuteReadFileWithSpecialCharacters", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": "/test/file with spaces!@#$.txt",
			},
		}

		expectedContent := []byte("content with special characters")
		mockFileOps.On("ReadFile", "/test/file with spaces!@#$.txt").Return(expectedContent, nil)

		result, err := actionExecutor.Execute(ctx, action)

		assert.NoError(t, err)
		assert.Equal(t, expectedContent, result)
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteWebSearchWithRetry", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "WebSearch",
			Params: map[string]interface{}{
				"query": "retry test",
			},
		}

		mockWebSearch.On("Search", "retry test").
			Return([]string{}, errors.New("temporary error")).Once().
			On("Search", "retry test").
			Return([]string{"retry result"}, nil).Once()

		result, err := actionExecutor.Execute(ctx, action)

		assert.NoError(t, err)
		assert.Equal(t, []string{"retry result"}, result)
		mockWebSearch.AssertExpectations(t)
	})

	t.Run("ExecuteReadFileConcurrently", func(t *testing.T) {
		ctx := context.Background()
		action := models.Action{
			Type: "ReadFile",
			Params: map[string]interface{}{
				"path": "/test/concurrent_file.txt",
			},
		}

		expectedContent := []byte("concurrent content")
		mockFileOps.On("ReadFile", "/test/concurrent_file.txt").Return(expectedContent, nil)

		var wg sync.WaitGroup
		var mu sync.Mutex
		var results []interface{}
		var errs []error

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := actionExecutor.Execute(ctx, action)
				mu.Lock()
				results = append(results, result)
				errs = append(errs, err)
				mu.Unlock()
			}()
		}

		wg.Wait()

		for _, err := range errs {
			assert.NoError(t, err)
		}
		for _, result := range results {
			assert.Equal(t, expectedContent, result)
		}
		mockFileOps.AssertExpectations(t)
	})

	t.Run("ExecuteReadFileWithDifferentEncodings", func(t *testing.T) {
		testCases := []struct {
			encoding string
			content  []byte
		}{
			{"UTF-8", []byte("UTF-8 content")},
			{"UTF-16", []byte{0xFF, 0xFE, 0x55, 0x00, 0x54, 0x00, 0x46, 0x00, 0x2D, 0x00, 0x31, 0x00, 0x36, 0x00}},
			{"ASCII", []byte("ASCII content")},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("Encoding_%s", tc.encoding), func(t *testing.T) {
				ctx := context.Background()
				action := models.Action{
					Type: "ReadFile",
					Params: map[string]interface{}{
						"path":     fmt.Sprintf("/test/%s_file.txt", tc.encoding),
						"encoding": tc.encoding,
					},
				}

				mockFileOps.On("ReadFile", fmt.Sprintf("/test/%s_file.txt", tc.encoding)).Return(tc.content, nil)

				result, err := actionExecutor.Execute(ctx, action)

				assert.NoError(t, err)
				assert.Equal(t, tc.content, result)
				mockFileOps.AssertExpectations(t)
			})
		}
	})

	t.Run("ExecuteWriteFileWithDifferentPermissions", func(t *testing.T) {
		testCases := []struct {
			permissions uint32
		}{
			{0644},
			{0755},
			{0600},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("Permissions_%o", tc.permissions), func(t *testing.T) {
				ctx := context.Background()
				action := models.Action