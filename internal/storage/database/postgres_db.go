package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

type Project struct {
	ID          int64
	Name        string
	Description string
}

type Task struct {
	ID          int64
	ProjectID   int64
	Description string
	Status      string
}

type CodeExecution struct {
	ID     int64
	TaskID int64
	Code   string
	Result string
}

type LLMRequest struct {
	ID       int64
	TaskID   int64
	Prompt   string
	Response string
}

func NewPostgresDB(connectionString string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %v", err)
	}

	maxConns, _ := strconv.Atoi(os.Getenv("DB_MAX_CONNS"))
	minConns, _ := strconv.Atoi(os.Getenv("DB_MIN_CONNS"))
	maxConnLifetime, _ := time.ParseDuration(os.Getenv("DB_MAX_CONN_LIFETIME"))
	maxConnIdleTime, _ := time.ParseDuration(os.Getenv("DB_MAX_CONN_IDLE_TIME"))

	config.MaxConns = int32(maxConns)
	config.MinConns = int32(minConns)
	config.MaxConnLifetime = maxConnLifetime
	config.MaxConnIdleTime = maxConnIdleTime

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return &PostgresDB{pool: pool}, nil
}

func (db *PostgresDB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

func (db *PostgresDB) CreateProject(ctx context.Context, name string, description string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("project name cannot be empty")
	}

	var id int64
	err := db.pool.QueryRow(ctx,
		"INSERT INTO projects (name, description, created_at, updated_at) VALUES ($1, $2, NOW(), NOW()) RETURNING id",
		name, description).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create project: %v", err)
	}
	return id, nil
}

func (db *PostgresDB) GetProject(ctx context.Context, id int64) (*Project, error) {
	project := &Project{}
	err := db.pool.QueryRow(ctx,
		"SELECT id, name, description FROM projects WHERE id = $1 AND deleted_at IS NULL", id).Scan(&project.ID, &project.Name, &project.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %v", err)
	}
	return project, nil
}

func (db *PostgresDB) UpdateProject(ctx context.Context, id int64, name string, description string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	_, err := db.pool.Exec(ctx,
		"UPDATE projects SET name = $1, description = $2, updated_at = NOW() WHERE id = $3 AND deleted_at IS NULL",
		name, description, id)
	if err != nil {
		return fmt.Errorf("failed to update project: %v", err)
	}
	return nil
}

func (db *PostgresDB) DeleteProject(ctx context.Context, id int64) error {
	_, err := db.pool.Exec(ctx, "UPDATE projects SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("failed to soft delete project: %v", err)
	}
	return nil
}

func (db *PostgresDB) ListProjects(ctx context.Context, limit, offset int) ([]Project, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, name, description FROM projects WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %v", err)
	}
	defer rows.Close()

	var projects []Project

	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description); err != nil {
			return nil, fmt.Errorf("failed to scan project row: %v", err)
		}
		projects = append(projects, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project rows: %v", err)
	}

	return projects, nil
}

func (db *PostgresDB) CreateTask(ctx context.Context, projectID int64, description string, status string) (int64, error) {
	if description == "" {
		return 0, fmt.Errorf("task description cannot be empty")
	}
	if status == "" {
		return 0, fmt.Errorf("task status cannot be empty")
	}

	var id int64
	err := db.pool.QueryRow(ctx,
		"INSERT INTO tasks (project_id, description, status, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW()) RETURNING id",
		projectID, description, status).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create task: %v", err)
	}
	return id, nil
}

func (db *PostgresDB) GetTask(ctx context.Context, id int64) (*Task, error) {
	task := &Task{}
	err := db.pool.QueryRow(ctx,
		"SELECT id, project_id, description, status FROM tasks WHERE id = $1 AND deleted_at IS NULL", id).Scan(&task.ID, &task.ProjectID, &task.Description, &task.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %v", err)
	}
	return task, nil
}

func (db *PostgresDB) UpdateTask(ctx context.Context, id int64, description string, status string) error {
	if description == "" {
		return fmt.Errorf("task description cannot be empty")
	}
	if status == "" {
		return fmt.Errorf("task status cannot be empty")
	}

	_, err := db.pool.Exec(ctx,
		"UPDATE tasks SET description = $1, status = $2, updated_at = NOW() WHERE id = $3 AND deleted_at IS NULL",
		description, status, id)
	if err != nil {
		return fmt.Errorf("failed to update task: %v", err)
	}
	return nil
}

func (db *PostgresDB) DeleteTask(ctx context.Context, id int64) error {
	_, err := db.pool.Exec(ctx, "UPDATE tasks SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("failed to soft delete task: %v", err)
	}
	return nil
}

func (db *PostgresDB) ListTasksForProject(ctx context.Context, projectID int64, limit, offset int) ([]Task, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, project_id, description, status FROM tasks WHERE project_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3", projectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for project: %v", err)
	}
	defer rows.Close()

	var tasks []Task

	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Description, &t.Status); err != nil {
			return nil, fmt.Errorf("failed to scan task row: %v", err)
		}
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows: %v", err)
	}

	return tasks, nil
}

func (db *PostgresDB) CreateCodeExecution(ctx context.Context, taskID int64, code string, result string) (int64, error) {
	if code == "" {
		return 0, fmt.Errorf("code cannot be empty")
	}

	var id int64
	err := db.pool.QueryRow(ctx,
		"INSERT INTO code_executions (task_id, code, result, executed_at) VALUES ($1, $2, $3, NOW()) RETURNING id",
		taskID, code, result).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create code execution: %v", err)
	}
	return id, nil
}

func (db *PostgresDB) GetCodeExecution(ctx context.Context, id int64) (*CodeExecution, error) {
	execution := &CodeExecution{}
	err := db.pool.QueryRow(ctx,
		"SELECT id, task_id, code, result FROM code_executions WHERE id = $1", id).Scan(&execution.ID, &execution.TaskID, &execution.Code, &execution.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to get code execution: %v", err)
	}
	return execution, nil
}

func (db *PostgresDB) ListCodeExecutionsForTask(ctx context.Context, taskID int64, limit, offset int) ([]CodeExecution, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, task_id, code, result FROM code_executions WHERE task_id = $1 ORDER BY executed_at DESC LIMIT $2 OFFSET $3", taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list code executions for task: %v", err)
	}
	defer rows.Close()

	var executions []CodeExecution

	for rows.Next() {
		var e CodeExecution
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Code, &e.Result); err != nil {
			return nil, fmt.Errorf("failed to scan code execution row: %v", err)
		}
		executions = append(executions, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating code execution rows: %v", err)
	}

	return executions, nil
}

func (db *PostgresDB) CreateLLMRequest(ctx context.Context, taskID int64, prompt string, response string) (int64, error) {
	if prompt == "" {
		return 0, fmt.Errorf("prompt cannot be empty")
	}

	var id int64
	err := db.pool.QueryRow(ctx,
		"INSERT INTO llm_requests (task_id, prompt, response, created_at) VALUES ($1, $2, $3, NOW()) RETURNING id",
		taskID, prompt, response).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create LLM request: %v", err)
	}
	return id, nil
}

func (db *PostgresDB) GetLLMRequest(ctx context.Context, id int64) (*LLMRequest, error) {
	request := &LLMRequest{}
	err := db.pool.QueryRow(ctx,
		"SELECT id, task_id, prompt, response FROM llm_requests WHERE id = $1", id).Scan(&request.ID, &request.TaskID, &request.Prompt, &request.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM request: %v", err)
	}
	return request, nil
}

func (db *PostgresDB) ListLLMRequestsForTask(ctx context.Context, taskID int64, limit, offset int) ([]LLMRequest, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, task_id, prompt, response FROM llm_requests WHERE task_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list LLM requests for task: %v", err)
	}
	defer rows.Close()

	var requests []LLMRequest

	for rows.Next() {
		var r LLMRequest
		if err := rows.Scan(&r.ID, &r.TaskID, &r.Prompt, &r.Response); err != nil {
			return nil, fmt.Errorf("failed to scan LLM request row: %v", err)
		}
		requests = append(requests, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating LLM request rows: %v", err)
	}

	return requests, nil
}