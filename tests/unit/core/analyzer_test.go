package core

import (
	"context"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockLLMService struct {
	mock.Mock
}

func (m *MockLLMService) AnalyzeRequirements(ctx context.Context, requirements string) (*models.AnalysisResult, error) {
	args := m.Called(ctx, requirements)
	return args.Get(0).(*models.AnalysisResult), args.Error(1)
}

func TestAnalyzer_Analyze(t *testing.T) {
	mockLLMService := new(MockLLMService)
	analyzer := NewAnalyzer(mockLLMService)

	ctx := context.Background()
	requirements := "Create a simple web server"

	t.Run("Successful analysis", func(t *testing.T) {
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Set up HTTP server"},
				{ID: "2", Description: "Implement request handler"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, requirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, requirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("LLM service error", func(t *testing.T) {
		mockLLMService.On("AnalyzeRequirements", ctx, requirements).Return((*models.AnalysisResult)(nil), assert.AnError)

		result, err := analyzer.Analyze(ctx, requirements)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Empty requirements", func(t *testing.T) {
		emptyRequirements := ""
		mockLLMService.On("AnalyzeRequirements", ctx, emptyRequirements).Return((*models.AnalysisResult)(nil), assert.AnError)

		result, err := analyzer.Analyze(ctx, emptyRequirements)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Large requirements", func(t *testing.T) {
		largeRequirements := "Create a complex distributed system with microservices, load balancing, and database sharding"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Design microservices architecture"},
				{ID: "2", Description: "Implement load balancing"},
				{ID: "3", Description: "Set up database sharding"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, largeRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, largeRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Malformed requirements", func(t *testing.T) {
		malformedRequirements := "123@#$%^&*()"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, malformedRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, malformedRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		assert.Empty(t, result.Tasks)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Long processing time", func(t *testing.T) {
		longRequirements := "Create a complex AI system with natural language processing and machine learning capabilities"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Design AI architecture"},
				{ID: "2", Description: "Implement NLP module"},
				{ID: "3", Description: "Develop machine learning algorithms"},
			},
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		mockLLMService.On("AnalyzeRequirements", ctxWithTimeout, longRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctxWithTimeout, longRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Context cancellation", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel the context immediately

		mockLLMService.On("AnalyzeRequirements", canceledCtx, requirements).Return((*models.AnalysisResult)(nil), context.Canceled)

		result, err := analyzer.Analyze(canceledCtx, requirements)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Nil(t, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Partial analysis result", func(t *testing.T) {
		partialRequirements := "Create a web application with frontend and backend"
		partialResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Set up frontend framework"},
			},
			PartialAnalysis: true,
		}

		mockLLMService.On("AnalyzeRequirements", ctx, partialRequirements).Return(partialResult, nil)

		result, err := analyzer.Analyze(ctx, partialRequirements)

		assert.NoError(t, err)
		assert.Equal(t, partialResult, result)
		assert.True(t, result.PartialAnalysis)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with special characters", func(t *testing.T) {
		specialRequirements := "Create a system that handles UTF-8 characters: áéíóú ñ 你好 こんにちは"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Implement UTF-8 encoding support"},
				{ID: "2", Description: "Test with various language characters"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, specialRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, specialRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with code snippets", func(t *testing.T) {
		codeRequirements := "Create a function that sorts an array: ```func sortArray(arr []int) []int { // TODO: Implement sorting algorithm }```"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Implement sorting algorithm"},
				{ID: "2", Description: "Test sorting function with various inputs"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, codeRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, codeRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with multiple languages", func(t *testing.T) {
		multiLangRequirements := "Create a polyglot application using Python, JavaScript, and Go"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Set up Python backend"},
				{ID: "2", Description: "Implement JavaScript frontend"},
				{ID: "3", Description: "Create Go microservice"},
				{ID: "4", Description: "Integrate all components"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, multiLangRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, multiLangRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with conflicting tasks", func(t *testing.T) {
		conflictingRequirements := "Create a high-performance system with both SQL and NoSQL databases"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Design database schema for SQL"},
				{ID: "2", Description: "Set up NoSQL database"},
				{ID: "3", Description: "Implement data access layer"},
				{ID: "4", Description: "Optimize queries for both SQL and NoSQL"},
			},
			Conflicts: []string{"Potential performance issues with mixed database types"},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, conflictingRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, conflictingRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		assert.NotEmpty(t, result.Conflicts)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with dependencies", func(t *testing.T) {
		dependentRequirements := "Build a web application with user authentication and profile management"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Implement user registration"},
				{ID: "2", Description: "Create login functionality"},
				{ID: "3", Description: "Develop user profile management", DependsOn: []string{"1", "2"}},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, dependentRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, dependentRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		assert.NotEmpty(t, result.Tasks[2].DependsOn)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Extremely large requirements", func(t *testing.T) {
		largeReqs := make([]byte, 1<<20) // 1 MB of data
		for i := range largeReqs {
			largeReqs[i] = 'a'
		}
		largeRequirements := string(largeReqs)

		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Process large input"},
				{ID: "2", Description: "Optimize for memory usage"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, largeRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, largeRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with non-ASCII characters", func(t *testing.T) {
		nonASCIIRequirements := "创建一个多语言支持的应用程序 with الدعم العربي and Русская поддержка"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Implement multi-language support"},
				{ID: "2", Description: "Add Chinese language pack"},
				{ID: "3", Description: "Add Arabic language pack"},
				{ID: "4", Description: "Add Russian language pack"},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, nonASCIIRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, nonASCIIRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with multiple nested dependencies", func(t *testing.T) {
		complexRequirements := "Build a microservices architecture with API gateway, service discovery, and circuit breaker"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Design microservices architecture"},
				{ID: "2", Description: "Implement API gateway", DependsOn: []string{"1"}},
				{ID: "3", Description: "Set up service discovery", DependsOn: []string{"1", "2"}},
				{ID: "4", Description: "Implement circuit breaker", DependsOn: []string{"1", "2", "3"}},
			},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, complexRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, complexRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		assert.Equal(t, []string{"1", "2", "3"}, result.Tasks[3].DependsOn)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with cyclic dependencies", func(t *testing.T) {
		cyclicRequirements := "Build a system with circular dependencies"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Task A", DependsOn: []string{"3"}},
				{ID: "2", Description: "Task B", DependsOn: []string{"1"}},
				{ID: "3", Description: "Task C", DependsOn: []string{"2"}},
			},
			Conflicts: []string{"Cyclic dependencies detected"},
		}

		mockLLMService.On("AnalyzeRequirements", ctx, cyclicRequirements).Return(expectedResult, nil)

		result, err := analyzer.Analyze(ctx, cyclicRequirements)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		assert.NotEmpty(t, result.Conflicts)
		mockLLMService.AssertExpectations(t)
	})

	t.Run("Requirements with ambiguous tasks", func(t *testing.T) {
		ambiguousRequirements := "Create a system that is both fast and comprehensive"
		expectedResult := &models.AnalysisResult{
			Tasks: []models.Task{
				{ID: "1", Description: "Optimize system performance