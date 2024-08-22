package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/Nero7991/devlm/internal/core/action"
	"github.com/Nero7991/devlm/internal/core/codeexecution"
	"github.com/Nero7991/devlm/internal/core/llm"
	"github.com/Nero7991/devlm/internal/core/models"
)

type Orchestrator struct {
	llmService     *llm.Service
	actionExecutor *action.Executor
	codeExecutor   *codeexecution.Executor
}

func NewOrchestrator(llmService *llm.Service, actionExecutor *action.Executor, codeExecutor *codeexecution.Executor) (*Orchestrator, error) {
	if llmService == nil || actionExecutor == nil || codeExecutor == nil {
		return nil, fmt.Errorf("invalid input parameters: services and executors cannot be nil")
	}
	return &Orchestrator{
		llmService:     llmService,
		actionExecutor: actionExecutor,
		codeExecutor:   codeExecutor,
	}, nil
}

func (o *Orchestrator) ProcessProject(ctx context.Context, projectDir string) error {
	devFilePath := filepath.Join(projectDir, "dev.txt")
	content, err := ioutil.ReadFile(devFilePath)
	if err != nil {
		return fmt.Errorf("failed to read dev.txt: %w", err)
	}

	requirements, err := o.llmService.AnalyzeRequirements(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to analyze requirements: %w", err)
	}

	tasks, err := o.llmService.GenerateTasks(ctx, requirements)
	if err != nil {
		return fmt.Errorf("failed to generate tasks: %w", err)
	}

	taskGraph, err := o.buildTaskGraph(tasks)
	if err != nil {
		return fmt.Errorf("failed to build task graph: %w", err)
	}

	state := models.ProjectState{
		Variables:      make(map[string]string),
		Files:          make(map[string]models.FileState),
		CompletedTasks: []string{},
	}

	err = o.executeTasks(ctx, taskGraph, projectDir, &state)
	if err != nil {
		// Partial completion handling
		log.Printf("Project processing completed partially: %v", err)
		return o.SaveProjectState(ctx, projectDir, state)
	}

	return o.SaveProjectState(ctx, projectDir, state)
}

func (o *Orchestrator) buildTaskGraph(tasks []models.Task) (*models.TaskGraph, error) {
	graph := &models.TaskGraph{
		Tasks:        make(map[string]models.Task),
		Dependencies: make(map[string][]string),
	}

	for _, task := range tasks {
		graph.Tasks[task.ID] = task
		graph.Dependencies[task.ID] = task.Dependencies
	}

	// Validate for cyclic dependencies
	if hasCycle := o.detectCycle(graph); hasCycle {
		return nil, fmt.Errorf("cyclic dependency detected in task graph")
	}

	return graph, nil
}

func (o *Orchestrator) detectCycle(graph *models.TaskGraph) bool {
	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var dfs func(string) bool
	dfs = func(node string) bool {
		visited[node] = true
		stack[node] = true

		for _, neighbor := range graph.Dependencies[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if stack[neighbor] {
				return true
			}
		}

		stack[node] = false
		return false
	}

	for node := range graph.Tasks {
		if !visited[node] {
			if dfs(node) {
				return true
			}
		}
	}

	return false
}

func (o *Orchestrator) executeTasks(ctx context.Context, taskGraph *models.TaskGraph, projectDir string, state *models.ProjectState) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(taskGraph.Tasks))
	taskChan := make(chan models.Task, len(taskGraph.Tasks))

	// Sort tasks by priority
	sortedTasks := o.sortTasksByPriority(taskGraph)

	for _, task := range sortedTasks {
		taskChan <- task
	}
	close(taskChan)

	workerCount := 5
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				if err := o.processTask(ctx, task, projectDir, state); err != nil {
					errChan <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("task processing errors: %v", errors)
	}

	return nil
}

func (o *Orchestrator) sortTasksByPriority(taskGraph *models.TaskGraph) []models.Task {
	sortedTasks := make([]models.Task, 0, len(taskGraph.Tasks))
	for _, task := range taskGraph.Tasks {
		sortedTasks = append(sortedTasks, task)
	}

	// Sort tasks based on priority (assuming higher number means higher priority)
	sort.Slice(sortedTasks, func(i, j int) bool {
		return sortedTasks[i].Priority > sortedTasks[j].Priority
	})

	return sortedTasks
}

func (o *Orchestrator) processTask(ctx context.Context, task models.Task, projectDir string, state *models.ProjectState) error {
	maxRetries := 3
	var err error
	for retry := 0; retry < maxRetries; retry++ {
		err = o.executeTask(ctx, task, projectDir, state)
		if err == nil {
			state.CompletedTasks = append(state.CompletedTasks, task.ID)
			state.LastExecutedTask = task.ID
			return nil
		}
		log.Printf("Task %s failed (attempt %d/%d): %v", task.ID, retry+1, maxRetries, err)
		time.Sleep(time.Second * time.Duration(1<<uint(retry))) // Exponential backoff
	}
	return fmt.Errorf("task %s failed after %d retries: %w", task.ID, maxRetries, err)
}

func (o *Orchestrator) executeTask(ctx context.Context, task models.Task, projectDir string, state *models.ProjectState) error {
	code, err := o.llmService.GenerateCode(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to generate code for task %s: %w", task.ID, err)
	}

	result, err := o.codeExecutor.Execute(ctx, code, task.ExecutionEnvironment)
	if err != nil {
		return fmt.Errorf("failed to execute code for task %s: %w", task.ID, err)
	}

	analysis, err := o.llmService.AnalyzeExecutionResult(ctx, result)
	if err != nil {
		return fmt.Errorf("failed to analyze execution result for task %s: %w", task.ID, err)
	}

	if err := o.performActions(ctx, analysis, projectDir, state); err != nil {
		return fmt.Errorf("failed to perform actions for task %s: %w", task.ID, err)
	}

	return nil
}

func (o *Orchestrator) performActions(ctx context.Context, analysis models.ExecutionAnalysis, projectDir string, state *models.ProjectState) error {
	for _, action := range analysis.Actions {
		if err := o.executeAction(ctx, action, projectDir, state); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) executeAction(ctx context.Context, action models.Action, projectDir string, state *models.ProjectState) error {
	switch action.Type {
	case models.ActionTypeFileWrite:
		return o.executeFileWrite(ctx, action, projectDir, state)
	case models.ActionTypeFileRead:
		return o.executeFileRead(ctx, action, projectDir, state)
	case models.ActionTypeWebSearch:
		return o.executeWebSearch(ctx, action, state)
	case models.ActionTypeFileDelete:
		return o.executeFileDelete(ctx, action, projectDir, state)
	case models.ActionTypeDirectoryCreate:
		return o.executeDirectoryCreate(ctx, action, projectDir)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

func (o *Orchestrator) executeFileWrite(ctx context.Context, action models.Action, projectDir string, state *models.ProjectState) error {
	path := action.Params["path"].(string)
	content := action.Params["content"].(string)
	fullPath := filepath.Join(projectDir, path)
	if err := o.actionExecutor.WriteFile(ctx, fullPath, content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	state.Files[path] = models.FileState{
		Content:      content,
		LastModified: time.Now(),
	}
	return nil
}

func (o *Orchestrator) executeFileRead(ctx context.Context, action models.Action, projectDir string, state *models.ProjectState) error {
	path := action.Params["path"].(string)
	fullPath := filepath.Join(projectDir, path)
	content, err := o.actionExecutor.ReadFile(ctx, fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	state.Files[path] = models.FileState{
		Content:      content,
		LastModified: time.Now(),
	}
	return nil
}

func (o *Orchestrator) executeWebSearch(ctx context.Context, action models.Action, state *models.ProjectState) error {
	query := action.Params["query"].(string)
	results, err := o.actionExecutor.WebSearch(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to perform web search: %w", err)
	}
	state.Variables["last_search_query"] = query
	state.Variables["last_search_results"] = fmt.Sprintf("%v", results)
	return nil
}

func (o *Orchestrator) executeFileDelete(ctx context.Context, action models.Action, projectDir string, state *models.ProjectState) error {
	path := action.Params["path"].(string)
	fullPath := filepath.Join(projectDir, path)
	if err := o.actionExecutor.DeleteFile(ctx, fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	delete(state.Files, path)
	return nil
}

func (o *Orchestrator) executeDirectoryCreate(ctx context.Context, action models.Action, projectDir string) error {
	path := action.Params["path"].(string)
	fullPath := filepath.Join(projectDir, path)
	if err := o.actionExecutor.CreateDirectory(ctx, fullPath); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

func (o *Orchestrator) SaveProjectState(ctx context.Context, projectDir string, state models.ProjectState) error {
	stateFile := filepath.Join(projectDir, fmt.Sprintf("project_state_%s.json", time.Now().Format("20060102150405")))
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project state: %w", err)
	}

	if err := ioutil.WriteFile(stateFile, stateJSON, 0644); err != nil {
		return fmt.Errorf("failed to write project state file: %w", err)
	}

	return nil
}

func (o *Orchestrator) LoadProjectState(ctx context.Context, projectDir string) (models.ProjectState, error) {
	stateFiles, err := filepath.Glob(filepath.Join(projectDir, "project_state_*.json"))
	if err != nil {
		return models.ProjectState{}, fmt.Errorf("failed to list project state files: %w", err)
	}

	if len(stateFiles) == 0 {
		return models.ProjectState{}, fmt.Errorf("no project state files found")
	}

	sort.Strings(stateFiles)
	latestStateFile := stateFiles[len(stateFiles)-1]
	stateJSON, err := ioutil.ReadFile(latestStateFile)
	if err != nil {
		return models.ProjectState{}, fmt.Errorf("failed to read project state file: %w", err)
	}

	var state models.ProjectState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return models.ProjectState{}, fmt.Errorf("failed to unmarshal project state: %w", err)
	}

	return state, nil
}

func (o *Orchestrator) UpdateProjectState(ctx context.Context, projectDir string, updatedState models.ProjectState) error {
	currentState, err := o.LoadProjectState(ctx, projectDir)
	if err != nil {
		return fmt.Errorf("failed to load current project state: %w", err)
	}

	mergedState := o.mergeProjectStates(currentState, updatedState)

	if err := o.SaveProjectState(ctx, projectDir, mergedState); err != nil {
		return fmt.Errorf("failed to save updated project state: %w", err)
	}

	return nil
}

func (o *Orchestrator) mergeProjectStates(current, updated models.ProjectState) models.ProjectState {
	merged := current

	for k, v := range updated.Variables {
		merged.Variables[k] = v
	}

	for k, v := range updated.Files {
		if existingFile, ok := merged.Files[k]; ok {
			merged.Files[k] = o.mergeFileStates(existingFile, v)
		} else {
			merged.Files[k] = v
		}
	}

	merged.LastExecutedTask = updated.LastExecutedTask
	merged.CompletedTasks = append(merged.CompletedTasks, updated.CompletedTasks...)

	return merged
}

func (o *Orchestrator) mergeFileStates(existing, updated models.FileState) models.FileState {
	merged := existing

	if updated.Content != "" {
		// Implement diff-based merging for file contents
		merged.Content = o.mergeDiff(existing.Content, updated.Content)
	}

	if updated.LastModified.After(existing.LastModified) {
		merged.LastModified = updated.LastModified
	}

	return merged
}

func (o *Orchestrator) mergeDiff(existingContent, updatedContent string) string