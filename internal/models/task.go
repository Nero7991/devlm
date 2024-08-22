package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

type Task struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key"`
	ProjectID   uuid.UUID `gorm:"type:uuid;not null"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	Status      TaskStatus `gorm:"type:varchar(20);not null;default:'pending'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
	Priority    int `gorm:"type:int;default:1"`
	DueDate     *time.Time
}

func NewTask(projectID uuid.UUID, name string, description string) (*Task, error) {
	if err := validateTaskInput(name, description); err != nil {
		return nil, err
	}

	if projectID == uuid.Nil {
		return nil, errors.New("invalid project ID")
	}

	return &Task{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Priority:    1,
	}, nil
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	return nil
}

func (t *Task) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = time.Now()
	return nil
}

func (t *Task) Complete() error {
	if t.Status == TaskStatusCompleted {
		return errors.New("task is already completed")
	}
	t.Status = TaskStatusCompleted
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

func (t *Task) Fail(reason string) error {
	if t.Status == TaskStatusFailed {
		return errors.New("task is already marked as failed")
	}
	t.Status = TaskStatusFailed
	t.Description += "\nFailed: " + reason
	return nil
}

func (t *Task) SetInProgress() error {
	if t.Status == TaskStatusInProgress {
		return errors.New("task is already in progress")
	}
	t.Status = TaskStatusInProgress
	return nil
}

func (t *Task) UpdateStatus(status TaskStatus) error {
	if !isValidTaskStatus(status) {
		return errors.New("invalid task status")
	}
	t.Status = status
	if status == TaskStatusCompleted {
		now := time.Now()
		t.CompletedAt = &now
	}
	return nil
}

func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusCompleted
}

func (t *Task) GetDuration() time.Duration {
	if t.CompletedAt == nil {
		return time.Since(t.CreatedAt)
	}
	return t.CompletedAt.Sub(t.CreatedAt)
}

func (t *Task) GetProjectID() uuid.UUID {
	return t.ProjectID
}

func (t *Task) GetName() string {
	return t.Name
}

func (t *Task) GetDescription() string {
	return t.Description
}

func (t *Task) GetStatus() TaskStatus {
	return t.Status
}

func (t *Task) GetCreatedAt() time.Time {
	return t.CreatedAt
}

func (t *Task) GetCompletedAt() *time.Time {
	return t.CompletedAt
}

func (t *Task) SetPriority(priority int) error {
	if priority < 1 || priority > 5 {
		return errors.New("priority must be between 1 and 5")
	}
	t.Priority = priority
	return nil
}

func (t *Task) GetPriority() int {
	return t.Priority
}

func (t *Task) SetDueDate(dueDate time.Time) {
	t.DueDate = &dueDate
}

func (t *Task) GetDueDate() *time.Time {
	return t.DueDate
}

func (t *Task) IsOverdue() bool {
	return t.DueDate != nil && time.Now().After(*t.DueDate) && t.Status != TaskStatusCompleted
}

func validateTaskInput(name, description string) error {
	if name == "" {
		return errors.New("task name cannot be empty")
	}
	if len(name) > 255 {
		return errors.New("task name is too long (max 255 characters)")
	}
	if len(description) > 1000 {
		return errors.New("task description is too long (max 1000 characters)")
	}
	return nil
}

func isValidTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted, TaskStatusFailed:
		return true
	default:
		return false
	}
}

func GetTaskByID(db *gorm.DB, id uuid.UUID) (*Task, error) {
	var task Task
	result := db.First(&task, "id = ?", id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &task, nil
}

func GetTasksByProjectID(db *gorm.DB, projectID uuid.UUID, limit, offset int) ([]Task, error) {
	var tasks []Task
	result := db.Where("project_id = ?", projectID).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}
	return tasks, nil
}

func GetTasksByStatus(db *gorm.DB, projectID uuid.UUID, status TaskStatus, limit, offset int) ([]Task, error) {
	var tasks []Task
	result := db.Where("project_id = ? AND status = ?", projectID, status).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}
	return tasks, nil
}

func GetOverdueTasks(db *gorm.DB, projectID uuid.UUID, limit, offset int) ([]Task, error) {
	var tasks []Task
	result := db.Where("project_id = ? AND due_date < ? AND status != ?", projectID, time.Now(), TaskStatusCompleted).
		Order("due_date ASC").
		Limit(limit).
		Offset(offset).
		Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}
	return tasks, nil
}

func (t *Task) Save(db *gorm.DB) error {
	return db.Save(t).Error
}

func (t *Task) Delete(db *gorm.DB) error {
	return db.Delete(t).Error
}

func (t *Task) SoftDelete(db *gorm.DB) error {
	return db.Model(t).Update("deleted_at", time.Now()).Error
}