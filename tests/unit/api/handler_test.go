package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/api"
	"github.com/Nero7991/devlm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockLLMService struct {
	mock.Mock
}

func (m *MockLLMService) GenerateCode(prompt string) (string, error) {
	args := m.Called(prompt)
	return args.String(0), args.Error(1)
}

type MockExecutionService struct {
	mock.Mock
}

func (m *MockExecutionService) ExecuteCode(code string, language string) (string, error) {
	args := m.Called(code, language)
	return args.String(0), args.Error(1)
}

type MockFileService struct {
	mock.Mock
}

func (m *MockFileService) ReadFile(path string) (string, error) {
	args := m.Called(path)
	return args.String(0), args.Error(1)
}

func (m *MockFileService) WriteFile(path string, content string) error {
	args := m.Called(path, content)
	return args.Error(0)
}

type MockSearchService struct {
	mock.Mock
}

func (m *MockSearchService) Search(query string, page int) ([]string, error) {
	args := m.Called(query, page)
	return args.Get(0).([]string), args.Error(1)
}

func TestHandleGenerateCode(t *testing.T) {
	mockLLMService := new(MockLLMService)
	handler := api.NewHandler(mockLLMService, nil, nil, nil)

	t.Run("successful code generation", func(t *testing.T) {
		mockLLMService.On("GenerateCode", "test prompt").Return("generated code", nil)

		req, err := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"test prompt"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response models.GenerateCodeResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "generated code", response.GeneratedCode)

		mockLLMService.AssertExpectations(t)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/generate", bytes.NewBufferString(`invalid json`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("LLM service error", func(t *testing.T) {
		mockLLMService.On("GenerateCode", "error prompt").Return("", errors.New("LLM service error"))

		req, err := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"error prompt"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		mockLLMService.AssertExpectations(t)
	})

	t.Run("empty prompt", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":""}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("very long prompt", func(t *testing.T) {
		longPrompt := string(make([]byte, 10001))
		req, err := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"`+longPrompt+`"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("special characters in prompt", func(t *testing.T) {
		mockLLMService.On("GenerateCode", "prompt with !@#$%^&*()").Return("generated code", nil)

		req, err := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"prompt with !@#$%^&*()"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		mockLLMService.AssertExpectations(t)
	})

	t.Run("rate limiting scenario", func(t *testing.T) {
		mockLLMService.On("GenerateCode", "test prompt").Return("generated code", nil)

		for i := 0; i < 5; i++ {
			req, _ := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"test prompt"}`))
			rr := httptest.NewRecorder()
			handler.HandleGenerateCode(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		}

		req, _ := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"test prompt"}`))
		rr := httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)

		time.Sleep(1 * time.Second)

		req, _ = http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"test prompt"}`))
		rr = httptest.NewRecorder()
		handler.HandleGenerateCode(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		mockLLMService.AssertExpectations(t)
	})

	t.Run("concurrent requests", func(t *testing.T) {
		mockLLMService.On("GenerateCode", mock.Anything).Return("generated code", nil)

		concurrentRequests := 10
		done := make(chan bool)

		for i := 0; i < concurrentRequests; i++ {
			go func() {
				req, _ := http.NewRequest("POST", "/generate", bytes.NewBufferString(`{"prompt":"concurrent test"}`))
				rr := httptest.NewRecorder()
				handler.HandleGenerateCode(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
				done <- true
			}()
		}

		for i := 0; i < concurrentRequests; i++ {
			<-done
		}

		mockLLMService.AssertNumberOfCalls(t, "GenerateCode", concurrentRequests)
	})
}

func TestHandleExecuteCode(t *testing.T) {
	mockExecutionService := new(MockExecutionService)
	handler := api.NewHandler(nil, mockExecutionService, nil, nil)

	t.Run("successful code execution", func(t *testing.T) {
		mockExecutionService.On("ExecuteCode", "print('Hello, World!')", "python").Return("Hello, World!", nil)

		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"print('Hello, World!')","language":"python"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response models.ExecuteCodeResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!", response.Output)

		mockExecutionService.AssertExpectations(t)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`invalid json`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("execution service error", func(t *testing.T) {
		mockExecutionService.On("ExecuteCode", "invalid code", "python").Return("", errors.New("execution error"))

		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"invalid code","language":"python"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		mockExecutionService.AssertExpectations(t)
	})

	t.Run("unsupported language", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"print('Hello')","language":"unsupported"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("execute JavaScript code", func(t *testing.T) {
		mockExecutionService.On("ExecuteCode", "console.log('Hello, JavaScript!')", "javascript").Return("Hello, JavaScript!", nil)

		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"console.log('Hello, JavaScript!')","language":"javascript"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response models.ExecuteCodeResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "Hello, JavaScript!", response.Output)

		mockExecutionService.AssertExpectations(t)
	})

	t.Run("execute Go code", func(t *testing.T) {
		mockExecutionService.On("ExecuteCode", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, Go!\")\n}", "go").Return("Hello, Go!", nil)

		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, Go!\")\n}","language":"go"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response models.ExecuteCodeResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "Hello, Go!", response.Output)

		mockExecutionService.AssertExpectations(t)
	})

	t.Run("execution timeout", func(t *testing.T) {
		mockExecutionService.On("ExecuteCode", "while True: pass", "python").Return("", errors.New("execution timeout"))

		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"while True: pass","language":"python"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusRequestTimeout, rr.Code)

		mockExecutionService.AssertExpectations(t)
	})

	t.Run("memory limit exceeded", func(t *testing.T) {
		mockExecutionService.On("ExecuteCode", "a = [0] * 1000000000", "python").Return("", errors.New("memory limit exceeded"))

		req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(`{"code":"a = [0] * 1000000000","language":"python"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleExecuteCode(rr, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)

		mockExecutionService.AssertExpectations(t)
	})
}

func TestHandleFileOperation(t *testing.T) {
	mockFileService := new(MockFileService)
	handler := api.NewHandler(nil, nil, mockFileService, nil)

	t.Run("successful file read", func(t *testing.T) {
		mockFileService.On("ReadFile", "/test/file.txt").Return("file content", nil)

		req, err := http.NewRequest("POST", "/file", bytes.NewBufferString(`{"operation":"read","path":"/test/file.txt"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleFileOperation(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response models.FileOperationResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "file content", response.Content)

		mockFileService.AssertExpectations(t)
	})

	t.Run("successful file write", func(t *testing.T) {
		mockFileService.On("WriteFile", "/test/file.txt", "new content").Return(nil)

		req, err := http.NewRequest("POST", "/file", bytes.NewBufferString(`{"operation":"write","path":"/test/file.txt","content":"new content"}`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleFileOperation(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		mockFileService.AssertExpectations(t)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/file", bytes.NewBufferString(`invalid json`))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.HandleFileOperation(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("file service error", func(t *testing.T) {
		mockFileService.On("ReadFile", "/error/file.txt