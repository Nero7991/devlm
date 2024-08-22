package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Nero7991/devlm/internal/models"
	"github.com/Nero7991/devlm/internal/services"
	"github.com/gorilla/mux"
)

type TaskHandler struct {
	taskService *services.TaskService
	cache       *services.CacheService
}

func NewTaskHandler(taskService *services.TaskService, cache *services.CacheService) (*TaskHandler, error) {
	if taskService == nil {
		return nil, errors.New("taskService cannot be nil")
	}
	if cache == nil {
		return nil, errors.New("cache cannot be nil")
	}
	return &TaskHandler{
		taskService: taskService,
		cache:       cache,
	}, nil
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding task: %v", err)
		return
	}

	if err := validateTask(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	createdTask, err := h.taskService.CreateTask(task)
	if err != nil {
		http.Error(w, "Failed to create task: "+err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating task: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTask)
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := getTaskIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cacheKey := "task:" + strconv.Itoa(taskID)
	cachedTask, err := h.cache.Get(cacheKey)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cachedTask)
		return
	}

	task, err := h.taskService.GetTask(taskID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTaskNotFound):
			http.Error(w, "Task not found", http.StatusNotFound)
		case errors.Is(err, services.ErrDatabaseError):
			http.Error(w, "Database error", http.StatusInternalServerError)
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		log.Printf("Error getting task: %v", err)
		return
	}

	taskJSON, err := json.Marshal(task)
	if err == nil {
		h.cache.Set(cacheKey, taskJSON, time.Minute*5)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := getTaskIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var updatedTask models.Task
	err = json.NewDecoder(r.Body).Decode(&updatedTask)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding updated task: %v", err)
		return
	}

	if err := validateTask(&updatedTask); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updatedTask.ID = taskID
	task, err := h.taskService.UpdateTask(updatedTask)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTaskNotFound):
			http.Error(w, "Task not found", http.StatusNotFound)
		case errors.Is(err, services.ErrDatabaseError):
			http.Error(w, "Database error", http.StatusInternalServerError)
		default:
			http.Error(w, "Failed to update task: "+err.Error(), http.StatusInternalServerError)
		}
		log.Printf("Error updating task: %v", err)
		return
	}

	h.cache.Delete("task:" + strconv.Itoa(taskID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := getTaskIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	confirm := r.URL.Query().Get("confirm")
	if confirm != "true" {
		http.Error(w, "Confirmation required. Add ?confirm=true to the request.", http.StatusBadRequest)
		return
	}

	err = h.taskService.DeleteTask(taskID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTaskNotFound):
			http.Error(w, "Task not found", http.StatusNotFound)
		case errors.Is(err, services.ErrDatabaseError):
			http.Error(w, "Database error", http.StatusInternalServerError)
		default:
			http.Error(w, "Failed to delete task: "+err.Error(), http.StatusInternalServerError)
		}
		log.Printf("Error deleting task: %v", err)
		return
	}

	h.cache.Delete("task:" + strconv.Itoa(taskID))

	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	page, pageSize := getPaginationParams(r)

	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	filter := r.URL.Query().Get("filter")

	cacheKey := "tasks:" + strconv.Itoa(page) + ":" + strconv.Itoa(pageSize) + ":" + sortBy + ":" + sortOrder + ":" + filter
	cachedTasks, err := h.cache.Get(cacheKey)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cachedTasks)
		return
	}

	tasks, totalCount, err := h.taskService.ListTasks(page, pageSize, sortBy, sortOrder, filter)
	if err != nil {
		http.Error(w, "Failed to list tasks: "+err.Error(), http.StatusInternalServerError)
		log.Printf("Error listing tasks: %v", err)
		return
	}

	response := struct {
		Tasks      []models.Task `json:"tasks"`
		TotalCount int           `json:"totalCount"`
		Page       int           `json:"page"`
		PageSize   int           `json:"pageSize"`
	}{
		Tasks:      tasks,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}

	responseJSON, err := json.Marshal(response)
	if err == nil {
		h.cache.Set(cacheKey, responseJSON, time.Minute*5)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getTaskIDFromRequest(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		return 0, errors.New("invalid task ID")
	}
	if id <= 0 {
		return 0, errors.New("task ID must be a positive integer")
	}
	return id, nil
}

func getPaginationParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	return page, pageSize
}

func validateTask(task *models.Task) error {
	if task.Title == "" {
		return errors.New("task title is required")
	}
	if len(task.Title) > 255 {
		return errors.New("task title is too long (max 255 characters)")
	}
	if task.Description != nil && len(*task.Description) > 1000 {
		return errors.New("task description is too long (max 1000 characters)")
	}
	if task.DueDate != nil && task.DueDate.Before(time.Now()) {
		return errors.New("due date cannot be in the past")
	}
	if task.Status != "" && !isValidStatus(task.Status) {
		return errors.New("invalid task status")
	}
	return nil
}

func isValidStatus(status string) bool {
	validStatuses := []string{"todo", "in_progress", "done"}
	for _, s := range validStatuses {
		if status == s {
			return true
		}
	}
	return false
}