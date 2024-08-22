package prompts

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Nero7991/devlm/internal/llm/prompts/templates"
)

type PromptGenerator struct {
	templates *templates.PromptTemplates
}

func NewPromptGenerator(opts ...Option) *PromptGenerator {
	pg := &PromptGenerator{
		templates: templates.NewPromptTemplates(),
	}

	for _, opt := range opts {
		opt(pg)
	}

	return pg
}

type Option func(*PromptGenerator)

func WithCustomTemplates(customTemplates *templates.PromptTemplates) Option {
	return func(pg *PromptGenerator) {
		pg.templates = customTemplates
	}
}

func (pg *PromptGenerator) GenerateInitialPrompt(requirements string) (string, error) {
	if requirements == "" {
		return "", fmt.Errorf("requirements cannot be empty")
	}
	return pg.templates.InitialPrompt(requirements), nil
}

func (pg *PromptGenerator) GenerateCodeGenerationPrompt(context, task string, options ...string) string {
	prompt := pg.templates.CodeGenerationPrompt(context, task)
	for _, opt := range options {
		prompt += fmt.Sprintf("\nAdditional option: %s", opt)
	}
	return prompt
}

func (pg *PromptGenerator) GenerateCodeReviewPrompt(code, requirements string, focusAreas ...string) string {
	prompt := pg.templates.CodeReviewPrompt(code, requirements)
	if len(focusAreas) > 0 {
		prompt += fmt.Sprintf("\nFocus areas for review: %s", strings.Join(focusAreas, ", "))
	}
	return prompt
}

func (pg *PromptGenerator) GenerateErrorAnalysisPrompt(code, errorMsg string) string {
	errorCategory := categorizeError(errorMsg)
	return pg.templates.ErrorAnalysisPrompt(code, errorMsg, errorCategory)
}

func categorizeError(errorMsg string) string {
	errorTypes := map[string]string{
		`(?i)syntax`: "SYNTAX_ERROR",
		`(?i)runtime`: "RUNTIME_ERROR",
		`(?i)logic`: "LOGIC_ERROR",
		`(?i)type`: "TYPE_ERROR",
		`(?i)memory`: "MEMORY_ERROR",
		`(?i)io`: "IO_ERROR",
		`(?i)network`: "NETWORK_ERROR",
		`(?i)concurrency`: "CONCURRENCY_ERROR",
	}

	for pattern, category := range errorTypes {
		if matched, _ := regexp.MatchString(pattern, errorMsg); matched {
			return category
		}
	}
	return "UNCATEGORIZED"
}

func (pg *PromptGenerator) GenerateWebSearchPrompt(query string, searchEngine string) string {
	if searchEngine == "" {
		searchEngine = "default"
	}
	return pg.templates.WebSearchPrompt(query, searchEngine)
}

func (pg *PromptGenerator) GenerateFileOperationPrompt(operation, path, content string) (string, error) {
	validOperations := []string{"read", "write", "delete", "create", "update"}
	if !contains(validOperations, operation) {
		return "", fmt.Errorf("invalid operation: %s. Supported operations are: %s", operation, strings.Join(validOperations, ", "))
	}
	return pg.templates.FileOperationPrompt(operation, path, content), nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (pg *PromptGenerator) GenerateTestGenerationPrompt(code, requirements, framework string) string {
	if framework == "" {
		framework = "default"
	}
	return pg.templates.TestGenerationPrompt(code, requirements, framework)
}

func (pg *PromptGenerator) GenerateRequirementAnalysisPrompt(requirements string) string {
	categories := categorizeRequirements(requirements)
	return pg.templates.RequirementAnalysisPrompt(requirements, categories)
}

func categorizeRequirements(requirements string) []string {
	categories := []string{"FUNCTIONAL", "NON_FUNCTIONAL"}
	
	categoryPatterns := map[string]string{
		"PERFORMANCE": `(?i)performance|speed|efficiency|response time`,
		"SECURITY": `(?i)security|authentication|authorization|encryption`,
		"USABILITY": `(?i)usability|user experience|ui|ux`,
		"SCALABILITY": `(?i)scalability|load handling|concurrent users`,
		"RELIABILITY": `(?i)reliability|availability|fault tolerance`,
		"MAINTAINABILITY": `(?i)maintainability|code quality|documentation`,
		"COMPATIBILITY": `(?i)compatibility|interoperability|integration`,
	}

	for category, pattern := range categoryPatterns {
		if matched, _ := regexp.MatchString(pattern, requirements); matched {
			categories = append(categories, category)
		}
	}

	return categories
}

func (pg *PromptGenerator) GenerateTaskDecompositionPrompt(task string, level int) string {
	if level <= 0 {
		level = 1
	}
	return pg.templates.TaskDecompositionPrompt(task, level)
}

func (pg *PromptGenerator) GenerateProgressUpdatePrompt(completedTasks, remainingTasks []string, timeEstimates map[string]string) string {
	completedTasksStr := strings.Join(completedTasks, ", ")
	remainingTasksStr := strings.Join(remainingTasks, ", ")
	
	var estimatesStr string
	for task, estimate := range timeEstimates {
		estimatesStr += fmt.Sprintf("%s: %s, ", task, estimate)
	}
	estimatesStr = strings.TrimSuffix(estimatesStr, ", ")

	return pg.templates.ProgressUpdatePrompt(completedTasksStr, remainingTasksStr, estimatesStr)
}