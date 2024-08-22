package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ResultStatus string

const (
	ResultStatusPending   ResultStatus = "pending"
	ResultStatusCompleted ResultStatus = "completed"
	ResultStatusFailed    ResultStatus = "failed"
)

type Result struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	TaskID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Status    ResultStatus
	Output    string    `gorm:"type:text"`
	Error     string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"index"`
	UpdatedAt time.Time
}

func NewResult(taskID uuid.UUID) (*Result, error) {
	if taskID == uuid.Nil {
		return nil, errors.New("invalid task ID")
	}
	return &Result{
		ID:     uuid.New(),
		TaskID: taskID,
		Status: ResultStatusPending,
	}, nil
}

func (r *Result) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *Result) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Result) Complete(output string) error {
	if r.Status != ResultStatusPending {
		return errors.New("cannot complete a non-pending result")
	}
	r.Status = ResultStatusCompleted
	r.Output = output
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Result) Fail(err string) error {
	if r.Status != ResultStatusPending {
		return errors.New("cannot fail a non-pending result")
	}
	r.Status = ResultStatusFailed
	r.Error = err
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Result) IsPending() bool {
	return r.Status == ResultStatusPending
}

func (r *Result) IsCompleted() bool {
	return r.Status == ResultStatusCompleted
}

func (r *Result) IsFailed() bool {
	return r.Status == ResultStatusFailed
}

func (r *Result) GetTaskID() uuid.UUID {
	return r.TaskID
}

func (r *Result) GetStatus() ResultStatus {
	return r.Status
}

func (r *Result) GetOutput() string {
	return r.Output
}

func (r *Result) GetError() string {
	return r.Error
}

func (r *Result) GetCreatedAt() time.Time {
	return r.CreatedAt
}

func (r *Result) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}

func (r *Result) SetStatus(status ResultStatus) error {
	if !r.isValidStatus(status) {
		return ErrInvalidResultStatus
	}
	r.Status = status
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Result) SetOutput(output string) {
	r.Output = output
	r.UpdatedAt = time.Now().UTC()
}

func (r *Result) SetError(err string) {
	r.Error = err
	r.UpdatedAt = time.Now().UTC()
}

func (r *Result) isValidStatus(status ResultStatus) bool {
	switch status {
	case ResultStatusPending, ResultStatusCompleted, ResultStatusFailed:
		return true
	default:
		return false
	}
}

func GetResultByID(db *gorm.DB, id uuid.UUID) (*Result, error) {
	var result Result
	err := db.Where("id = ?", id).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func GetResultsByTaskID(db *gorm.DB, taskID uuid.UUID, limit, offset int) ([]Result, error) {
	var results []Result
	err := db.Where("task_id = ?", taskID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&results).Error
	return results, err
}

func DeleteResult(db *gorm.DB, id uuid.UUID) error {
	return db.Delete(&Result{}, id).Error
}

func (r *Result) Save(db *gorm.DB) error {
	return db.Save(r).Error
}

func (r *Result) GetDuration() time.Duration {
	if r.IsCompleted() || r.IsFailed() {
		return r.UpdatedAt.Sub(r.CreatedAt)
	}
	return time.Since(r.CreatedAt)
}

func GetResultCountByTaskID(db *gorm.DB, taskID uuid.UUID) (int64, error) {
	var count int64
	err := db.Model(&Result{}).Where("task_id = ?", taskID).Count(&count).Error
	return count, err
}

var ErrInvalidResultStatus = errors.New("invalid result status")

func (r *Result) Validate() error {
	if r.TaskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if !r.isValidStatus(r.Status) {
		return ErrInvalidResultStatus
	}
	return nil
}

func GetResultsByStatus(db *gorm.DB, status ResultStatus, limit, offset int) ([]Result, error) {
	var results []Result
	err := db.Where("status = ?", status).Order("created_at DESC").Limit(limit).Offset(offset).Find(&results).Error
	return results, err
}

func GetResultsInDateRange(db *gorm.DB, startDate, endDate time.Time, limit, offset int) ([]Result, error) {
	var results []Result
	err := db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at DESC").Limit(limit).Offset(offset).Find(&results).Error
	return results, err
}

func (r *Result) SoftDelete(db *gorm.DB) error {
	return db.Model(r).Update("deleted_at", time.Now().UTC()).Error
}

func (r *Result) Restore(db *gorm.DB) error {
	return db.Model(r).Update("deleted_at", nil).Error
}