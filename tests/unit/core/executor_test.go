package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	args := m.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileSystem) WriteFile(filename string, data []byte) error {
	args := m.Called(filename, data)
	return args.Error(0)
}

type MockCodeExecutor struct {
	mock.Mock
}

func (m *MockCodeExecutor) Execute(ctx context.Context, code string) (*models.ExecutionResult, error) {
	args := m.Called(ctx, code)
	return args.Get(0).(*models.ExecutionResult), args.Error(1)
}

func TestExecutor_Execute(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeCodeExecution,
		Data: "print('Hello, World!')",
	}

	mockCodeExec.On("Execute", ctx, task.Data).Return(&models.ExecutionResult{
		Output: "Hello, World!\n",
		Error:  nil,
	}, nil)

	result, err := executor.Execute(ctx, task)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Hello, World!\n", result.Output)
	assert.Nil(t, result.Error)

	mockCodeExec.AssertExpectations(t)
}

func TestExecutor_Execute_FileOperation(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "read:test.txt",
	}

	mockFS.On("ReadFile", "test.txt").Return([]byte("File content"), nil)

	result, err := executor.Execute(ctx, task)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "File content", result.Output)
	assert.Nil(t, result.Error)

	mockFS.AssertExpectations(t)
}

func TestExecutor_Execute_UnsupportedTaskType(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: "UnsupportedType",
		Data: "Some data",
	}

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported task type")
}

func TestExecutor_Execute_CodeExecutionError(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeCodeExecution,
		Data: "invalid code",
	}

	mockCodeExec.On("Execute", ctx, task.Data).Return((*models.ExecutionResult)(nil), errors.New("execution error"))

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "execution error", err.Error())

	mockCodeExec.AssertExpectations(t)
}

func TestExecutor_Execute_FileOperationError(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "read:nonexistent.txt",
	}

	mockFS.On("ReadFile", "nonexistent.txt").Return([]byte{}, errors.New("file not found"))

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "file not found", err.Error())

	mockFS.AssertExpectations(t)
}

func TestExecutor_Execute_FileWriteOperation(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "write:test.txt:New content",
	}

	mockFS.On("WriteFile", "test.txt", []byte("New content")).Return(nil)

	result, err := executor.Execute(ctx, task)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "File written successfully", result.Output)
	assert.Nil(t, result.Error)

	mockFS.AssertExpectations(t)
}

func TestExecutor_Execute_FileWriteOperationError(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "write:test.txt:New content",
	}

	mockFS.On("WriteFile", "test.txt", []byte("New content")).Return(errors.New("write error"))

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "write error", err.Error())

	mockFS.AssertExpectations(t)
}

func TestExecutor_Execute_InvalidFileOperation(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "invalid:test.txt",
	}

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid file operation")
}

func TestExecutor_Execute_EmptyTask(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{}

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty task type")
}

func TestExecutor_Execute_LongRunningTask(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	task := &models.Task{
		Type: models.TaskTypeCodeExecution,
		Data: "long running code",
	}

	mockCodeExec.On("Execute", ctx, task.Data).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond)
	}).Return((*models.ExecutionResult)(nil), context.DeadlineExceeded)

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, context.DeadlineExceeded, err)

	mockCodeExec.AssertExpectations(t)
}

func TestExecutor_Execute_FileOperationWithLargeContent(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	largeContent := strings.Repeat("a", 1024*1024) // 1MB of content
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "write:large_file.txt:" + largeContent,
	}

	mockFS.On("WriteFile", "large_file.txt", []byte(largeContent)).Return(nil)

	result, err := executor.Execute(ctx, task)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "File written successfully", result.Output)
	assert.Nil(t, result.Error)

	mockFS.AssertExpectations(t)
}

func TestExecutor_Execute_ConcurrentTasks(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	numTasks := 10

	for i := 0; i < numTasks; i++ {
		task := &models.Task{
			Type: models.TaskTypeCodeExecution,
			Data: "print('Task " + string(rune('0'+i)) + "')",
		}

		mockCodeExec.On("Execute", ctx, task.Data).Return(&models.ExecutionResult{
			Output: "Task " + string(rune('0'+i)) + " executed\n",
			Error:  nil,
		}, nil)
	}

	var wg sync.WaitGroup
	results := make([]*models.ExecutionResult, numTasks)
	errs := make([]error, numTasks)

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task := &models.Task{
				Type: models.TaskTypeCodeExecution,
				Data: "print('Task " + string(rune('0'+i)) + "')",
			}
			results[i], errs[i] = executor.Execute(ctx, task)
		}(i)
	}

	wg.Wait()

	for i := 0; i < numTasks; i++ {
		assert.NoError(t, errs[i])
		assert.NotNil(t, results[i])
		assert.Contains(t, results[i].Output, "Task")
		assert.Contains(t, results[i].Output, "executed")
	}

	mockCodeExec.AssertExpectations(t)
}

func TestExecutor_Execute_ContextCancellation(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx, cancel := context.WithCancel(context.Background())
	task := &models.Task{
		Type: models.TaskTypeCodeExecution,
		Data: "long running code",
	}

	mockCodeExec.On("Execute", ctx, task.Data).Run(func(args mock.Arguments) {
		cancel()
		time.Sleep(50 * time.Millisecond)
	}).Return((*models.ExecutionResult)(nil), context.Canceled)

	result, err := executor.Execute(ctx, task)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, context.Canceled, err)

	mockCodeExec.AssertExpectations(t)
}

func TestExecutor_Execute_FileOperationWithSpecialCharacters(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	specialContent := "!@#$%^&*()_+{}|:<>?`~"
	task := &models.Task{
		Type: models.TaskTypeFileOperation,
		Data: "write:special_chars.txt:" + specialContent,
	}

	mockFS.On("WriteFile", "special_chars.txt", []byte(specialContent)).Return(nil)

	result, err := executor.Execute(ctx, task)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "File written successfully", result.Output)
	assert.Nil(t, result.Error)

	mockFS.AssertExpectations(t)
}

func TestExecutor_Execute_CodeExecutionWithResourceLimits(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockCodeExec := new(MockCodeExecutor)
	executor := NewExecutor(mockFS, mockCodeExec)

	ctx := context.Background()
	task := &models.Task{
		Type: models.TaskTypeCodeExecution,
		Data: "import time; time.sleep(10)",
	}

	mockCodeExec.On("Execute", ctx, task.Data).Return(&models.ExecutionResult{
		Output: "",
		Error:  errors.New("execution time limit exceeded"),
	}, nil)

	result, err := executor.Execute(ctx, task)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "execution time limit exceeded", result.Error.Error())

	mockCodeExec.AssertExpectations(t)
}