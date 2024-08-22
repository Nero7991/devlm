package distributor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Nero7991/devlm/internal/models"
	"github.com/Nero7991/devlm/internal/services/executor"
	"github.com/Nero7991/devlm/internal/services/filesystem"
	"github.com/Nero7991/devlm/internal/services/llm"
)

type TaskDistributor struct {
	maxConcurrency int
	taskQueue      chan models.Task
	workers        []*Worker
	llmService     *llm.Service
	executor       *executor.Service
	fsService      *filesystem.Service
	wg             sync.WaitGroup
}

func NewTaskDistributor(maxConcurrency int, llmService *llm.Service, executor *executor.Service, fsService *filesystem.Service) (*TaskDistributor, error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("invalid maxConcurrency: %d", maxConcurrency)
	}
	if llmService == nil || executor == nil || fsService == nil {
		return nil, fmt.Errorf("all services must be non-nil")
	}

	td := &TaskDistributor{
		maxConcurrency: maxConcurrency,
		taskQueue:      make(chan models.Task, maxConcurrency*2),
		workers:        make([]*Worker, maxConcurrency),
		llmService:     llmService,
		executor:       executor,
		fsService:      fsService,
	}

	for i := 0; i < maxConcurrency; i++ {
		td.workers[i] = NewWorker(i, td.taskQueue, llmService, executor, fsService)
	}

	return td, nil
}

func (td *TaskDistributor) Start(ctx context.Context) {
	td.wg.Add(len(td.workers))
	for _, worker := range td.workers {
		go func(w *Worker) {
			defer td.wg.Done()
			if err := w.Start(ctx); err != nil {
				log.Printf("Error starting worker: %v", err)
			}
		}(worker)
	}
}

func (td *TaskDistributor) Stop() {
	close(td.taskQueue)
	done := make(chan struct{})
	go func() {
		td.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers stopped successfully")
	case <-time.After(30 * time.Second):
		log.Println("Timeout waiting for workers to stop")
	}
}

func (td *TaskDistributor) DistributeTasks(tasks []models.Task) error {
	batchSize := 10
	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		batch := tasks[i:end]

		for _, task := range batch {
			select {
			case td.taskQueue <- task:
			default:
				return fmt.Errorf("task queue is full")
			}
		}
	}
	return nil
}

type Worker struct {
	id         int
	taskQueue  <-chan models.Task
	llmService *llm.Service
	executor   *executor.Service
	fsService  *filesystem.Service
	config     WorkerConfig
}

type WorkerConfig struct {
	MaxRetries        int
	RetryDelay        time.Duration
	TaskTimeout       time.Duration
	ErrorLogThreshold int
}

func NewWorker(id int, taskQueue <-chan models.Task, llmService *llm.Service, executor *executor.Service, fsService *filesystem.Service) *Worker {
	return &Worker{
		id:         id,
		taskQueue:  taskQueue,
		llmService: llmService,
		executor:   executor,
		fsService:  fsService,
		config: WorkerConfig{
			MaxRetries:        3,
			RetryDelay:        time.Second * 5,
			TaskTimeout:       time.Minute * 5,
			ErrorLogThreshold: 5,
		},
	}
}

func (w *Worker) Start(ctx context.Context) error {
	log.Printf("Worker %d started", w.id)
	for {
		select {
		case task, ok := <-w.taskQueue:
			if !ok {
				log.Printf("Worker %d shutting down", w.id)
				return nil
			}
			w.processTaskWithRetry(ctx, task)
		case <-ctx.Done():
			log.Printf("Worker %d context cancelled", w.id)
			return ctx.Err()
		}
	}
}

func (w *Worker) processTaskWithRetry(ctx context.Context, task models.Task) {
	var err error
	for attempt := 0; attempt < w.config.MaxRetries; attempt++ {
		taskCtx, cancel := context.WithTimeout(ctx, w.config.TaskTimeout)
		err = w.processTask(taskCtx, task)
		cancel()

		if err == nil {
			return
		}

		log.Printf("Worker %d failed to process task %s (attempt %d/%d): %v",
			w.id, task.ID, attempt+1, w.config.MaxRetries, err)

		if attempt < w.config.MaxRetries-1 {
			backoffDuration := time.Duration(1<<uint(attempt)) * w.config.RetryDelay
			time.Sleep(backoffDuration)
		}
	}

	task.Status = models.TaskStatusFailed
	task.Error = err.Error()
	log.Printf("Worker %d failed to process task %s after %d attempts: %v",
		w.id, task.ID, w.config.MaxRetries, err)
}

func (w *Worker) processTask(ctx context.Context, task models.Task) error {
	log.Printf("Worker %d processing task: %s", w.id, task.ID)

	llmResponse, err := w.llmService.ProcessTask(ctx, task)
	if err != nil {
		return fmt.Errorf("error processing task with LLM: %w", err)
	}

	if llmResponse.RequiresExecution {
		executionResult, err := w.executor.ExecuteCode(ctx, llmResponse.GeneratedCode)
		if err != nil {
			return fmt.Errorf("error executing code: %w", err)
		}
		task.ExecutionResult = executionResult
	}

	if llmResponse.RequiresFileOperation {
		err := w.fsService.PerformOperation(ctx, llmResponse.FileOperation)
		if err != nil {
			return fmt.Errorf("error performing file operation: %w", err)
		}
	}

	task.Status = models.TaskStatusCompleted
	task.Result = llmResponse.Result

	if err := w.persistTaskResult(task); err != nil {
		log.Printf("Warning: Failed to persist task result for task %s: %v", task.ID, err)
	}

	log.Printf("Worker %d completed task: %s", w.id, task.ID)
	return nil
}

func (w *Worker) persistTaskResult(task models.Task) error {
	// TODO: Implement task result persistence
	// This is a placeholder implementation
	log.Printf("Persisting task result for task %s", task.ID)
	return nil
}