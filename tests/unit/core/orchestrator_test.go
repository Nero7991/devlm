package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Nero7991/devlm/internal/core"
	"github.com/Nero7991/devlm/internal/models"
)

type MockAnalyzer struct {
	mock.Mock
}

func (m *MockAnalyzer) Analyze(ctx context.Context, requirements string) (*models.AnalysisResult, error) {
	args := m.Called(ctx, requirements)
	return args.Get(0).(*models.AnalysisResult), args.Error(1)
}

type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, task *models.Task) (*models.ExecutionResult, error) {
	args := m.Called(ctx, task)
	return args.Get(0).(*models.ExecutionResult), args.Error(1)
}

func TestOrchestrator_ProcessRequirements(t *testing.T) {
	t.Run("successful processing", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Sample requirements"

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Task 1"},
				{ID: "2", Description: "Task 2"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)

		executionResult1 := &models.ExecutionResult{TaskID: "1", Output: "Output 1"}
		executionResult2 := &models.ExecutionResult{TaskID: "2", Output: "Output 2"}

		mockExecutor.On("Execute", ctx, analysisResult.Tasks[0]).Return(executionResult1, nil)
		mockExecutor.On("Execute", ctx, analysisResult.Tasks[1]).Return(executionResult2, nil)

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Results, 2)
		assert.Equal(t, executionResult1, result.Results[0])
		assert.Equal(t, executionResult2, result.Results[1])

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("analyzer error", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Sample requirements"

		mockAnalyzer.On("Analyze", ctx, requirements).Return((*models.AnalysisResult)(nil), errors.New("analyzer error"))

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertNotCalled(t, "Execute")
	})

	t.Run("executor error", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Sample requirements"

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Task 1"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)
		mockExecutor.On("Execute", ctx, analysisResult.Tasks[0]).Return((*models.ExecutionResult)(nil), errors.New("executor error"))

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("empty requirements", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := ""

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "requirements cannot be empty", err.Error())

		mockAnalyzer.AssertNotCalled(t, "Analyze")
		mockExecutor.AssertNotCalled(t, "Execute")
	})

	t.Run("partial execution success", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Sample requirements"

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Task 1"},
				{ID: "2", Description: "Task 2"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)

		executionResult1 := &models.ExecutionResult{TaskID: "1", Output: "Output 1"}

		mockExecutor.On("Execute", ctx, analysisResult.Tasks[0]).Return(executionResult1, nil)
		mockExecutor.On("Execute", ctx, analysisResult.Tasks[1]).Return((*models.ExecutionResult)(nil), errors.New("task 2 execution failed"))

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Results, 1)
		assert.Equal(t, executionResult1, result.Results[0])

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("large number of tasks", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Sample requirements for large number of tasks"

		tasks := make([]*models.Task, 100)
		for i := 0; i < 100; i++ {
			tasks[i] = &models.Task{ID: fmt.Sprintf("%d", i+1), Description: fmt.Sprintf("Task %d", i+1)}
		}

		analysisResult := &models.AnalysisResult{
			Tasks: tasks,
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)

		for _, task := range tasks {
			mockExecutor.On("Execute", ctx, task).Return(&models.ExecutionResult{TaskID: task.ID, Output: fmt.Sprintf("Output %s", task.ID)}, nil)
		}

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Results, 100)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("context cancellation", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx, cancel := context.WithCancel(context.Background())
		requirements := "Sample requirements"

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Task 1"},
				{ID: "2", Description: "Task 2"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)

		mockExecutor.On("Execute", ctx, analysisResult.Tasks[0]).Run(func(args mock.Arguments) {
			cancel()
		}).Return((*models.ExecutionResult)(nil), context.Canceled)

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Nil(t, result)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("timeout scenario", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		requirements := "Sample requirements for timeout scenario"

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Long-running task"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)

		mockExecutor.On("Execute", ctx, analysisResult.Tasks[0]).Run(func(args mock.Arguments) {
			time.Sleep(200 * time.Millisecond)
		}).Return((*models.ExecutionResult)(nil), context.DeadlineExceeded)

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, context.DeadlineExceeded, err)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("requirements with special characters", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Sample requirements with special characters: !@#$%^&*()"

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Task with special chars"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)
		mockExecutor.On("Execute", ctx, analysisResult.Tasks[0]).Return(&models.ExecutionResult{TaskID: "1", Output: "Output"}, nil)

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Results, 1)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("requirements with multiple languages", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		requirements := "Requirements in English. Requisitos en español. требования на русском."

		analysisResult := &models.AnalysisResult{
			Tasks: []*models.Task{
				{ID: "1", Description: "Task in English"},
				{ID: "2", Description: "Tarea en español"},
				{ID: "3", Description: "Задача на русском"},
			},
		}

		mockAnalyzer.On("Analyze", ctx, requirements).Return(analysisResult, nil)
		for _, task := range analysisResult.Tasks {
			mockExecutor.On("Execute", ctx, task).Return(&models.ExecutionResult{TaskID: task.ID, Output: "Output"}, nil)
		}

		result, err := orchestrator.ProcessRequirements(ctx, requirements)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Results, 3)

		mockAnalyzer.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})
}

func TestOrchestrator_HandleTask(t *testing.T) {
	t.Run("successful task handling", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		task := &models.Task{ID: "1", Description: "Test task"}

		executionResult := &models.ExecutionResult{TaskID: "1", Output: "Task output"}

		mockExecutor.On("Execute", ctx, task).Return(executionResult, nil)

		result, err := orchestrator.HandleTask(ctx, task)

		assert.NoError(t, err)
		assert.Equal(t, executionResult, result)

		mockExecutor.AssertExpectations(t)
	})

	t.Run("executor error", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		task := &models.Task{ID: "1", Description: "Test task"}

		mockExecutor.On("Execute", ctx, task).Return((*models.ExecutionResult)(nil), errors.New("execution error"))

		result, err := orchestrator.HandleTask(ctx, task)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockExecutor.AssertExpectations(t)
	})

	t.Run("nil task", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()

		result, err := orchestrator.HandleTask(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "task cannot be nil", err.Error())

		mockExecutor.AssertNotCalled(t, "Execute")
	})

	t.Run("task with empty ID", func(t *testing.T) {
		mockAnalyzer := new(MockAnalyzer)
		mockExecutor := new(MockExecutor)

		orchestrator := core.NewOrchestrator(mockAnalyzer, mockExecutor)

		ctx := context.Background()
		task := &models.Task{ID: "", Description: "Test