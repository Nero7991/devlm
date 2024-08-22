package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectStatus string

const (
	StatusInProgress ProjectStatus = "In Progress"
	StatusCompleted  ProjectStatus = "Completed"
	StatusCancelled  ProjectStatus = "Cancelled"
	StatusOnHold     ProjectStatus = "On Hold"
)

type Project struct {
	ID          uuid.UUID     `json:"id" gorm:"type:uuid;primary_key"`
	Name        string        `json:"name" gorm:"not null"`
	Description string        `json:"description"`
	UserID      uuid.UUID     `json:"user_id" gorm:"type:uuid;not null"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Tasks       []Task        `json:"tasks" gorm:"foreignKey:ProjectID"`
	Status      ProjectStatus `json:"status" gorm:"default:'In Progress'"`
	Priority    int           `json:"priority" gorm:"default:1"`
}

func NewProject(name, description string, userID uuid.UUID) (*Project, error) {
	if name == "" {
		return nil, errors.New("project name cannot be empty")
	}
	if userID == uuid.Nil {
		return nil, errors.New("user ID cannot be nil")
	}
	if len(description) < 10 {
		return nil, errors.New("description must be at least 10 characters")
	}
	if len(description) > 1000 {
		return nil, errors.New("description must be less than 1000 characters")
	}
	return &Project{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      StatusInProgress,
		Priority:    1,
	}, nil
}

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *Project) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Project) GetTasks(db *gorm.DB) ([]Task, error) {
	var tasks []Task
	err := db.Where("project_id = ?", p.ID).Find(&tasks).Error
	return tasks, err
}

func (p *Project) AddTask(db *gorm.DB, task *Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}
	if task.Name == "" {
		return errors.New("task name cannot be empty")
	}
	task.ProjectID = p.ID
	return db.Create(task).Error
}

func (p *Project) UpdateStatus(db *gorm.DB, status ProjectStatus) error {
	if !isValidStatus(status) {
		return errors.New("invalid project status")
	}
	p.Status = status
	return db.Save(p).Error
}

func (p *Project) CalculateProgress(db *gorm.DB) (float64, error) {
	var completedTasks, totalTasks int64

	err := db.Model(&Task{}).Where("project_id = ?", p.ID).Count(&totalTasks).Error
	if err != nil {
		return 0, err
	}

	err = db.Model(&Task{}).Where("project_id = ? AND status = ?", p.ID, TaskStatusCompleted).Count(&completedTasks).Error
	if err != nil {
		return 0, err
	}

	if totalTasks == 0 {
		return 0, nil
	}

	progress := float64(completedTasks) / float64(totalTasks) * 100
	return progress, nil
}

func isValidStatus(status ProjectStatus) bool {
	switch status {
	case StatusInProgress, StatusCompleted, StatusCancelled, StatusOnHold:
		return true
	default:
		return false
	}
}

func GetProjectByID(db *gorm.DB, id uuid.UUID) (*Project, error) {
	var project Project
	err := db.Preload("Tasks").First(&project, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func GetProjectsByUserID(db *gorm.DB, userID uuid.UUID, limit, offset int) ([]Project, error) {
	var projects []Project
	err := db.Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&projects).Error
	return projects, err
}

func DeleteProject(db *gorm.DB, id uuid.UUID) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&Task{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Project{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (p *Project) UpdateProjectDetails(db *gorm.DB, name, description string) error {
	if name == "" {
		return errors.New("project name cannot be empty")
	}
	if len(description) < 10 {
		return errors.New("description must be at least 10 characters")
	}
	if len(description) > 1000 {
		return errors.New("description must be less than 1000 characters")
	}
	p.Name = name
	p.Description = description
	return db.Save(p).Error
}

func (p *Project) GetProjectStats(db *gorm.DB) (map[string]interface{}, error) {
	var totalTasks, completedTasks, inProgressTasks int64
	var err error

	if err = db.Model(&Task{}).Where("project_id = ?", p.ID).Count(&totalTasks).Error; err != nil {
		return nil, err
	}
	if err = db.Model(&Task{}).Where("project_id = ? AND status = ?", p.ID, TaskStatusCompleted).Count(&completedTasks).Error; err != nil {
		return nil, err
	}
	if err = db.Model(&Task{}).Where("project_id = ? AND status = ?", p.ID, TaskStatusInProgress).Count(&inProgressTasks).Error; err != nil {
		return nil, err
	}

	progress, err := p.CalculateProgress(db)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_tasks":       totalTasks,
		"completed_tasks":   completedTasks,
		"in_progress_tasks": inProgressTasks,
		"progress":          progress,
		"priority":          p.Priority,
	}, nil
}

func (p *Project) SetPriority(priority int) error {
	if priority < 1 || priority > 5 {
		return errors.New("priority must be between 1 and 5")
	}
	p.Priority = priority
	return nil
}

func (p *Project) GetTasksByStatus(db *gorm.DB, status TaskStatus) ([]Task, error) {
	var tasks []Task
	err := db.Where("project_id = ? AND status = ?", p.ID, status).Find(&tasks).Error
	return tasks, err
}

func (p *Project) GetOverdueTasks(db *gorm.DB) ([]Task, error) {
	var tasks []Task
	err := db.Where("project_id = ? AND due_date < ? AND status != ?", p.ID, time.Now(), TaskStatusCompleted).Find(&tasks).Error
	return tasks, err
}

func (p *Project) GetProjectDuration() time.Duration {
	return time.Since(p.CreatedAt)
}

func (p *Project) IsOverdue() bool {
	// Assuming a project is overdue if it's not completed and has been active for more than 30 days
	return p.Status != StatusCompleted && time.Since(p.CreatedAt) > 30*24*time.Hour
}