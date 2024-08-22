package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"log"

	"github.com/Nero7991/devlm/internal/models"
	"github.com/Nero7991/devlm/internal/services"
	"github.com/gorilla/mux"
)

const (
	maxPageSize = 100
)

type ProjectHandler struct {
	projectService *services.ProjectService
}

func NewProjectHandler(projectService *services.ProjectService) (*ProjectHandler, error) {
	if projectService == nil {
		return nil, models.ErrNilProjectService
	}
	return &ProjectHandler{
		projectService: projectService,
	}, nil
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var project models.Project
	err := json.NewDecoder(r.Body).Decode(&project)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding project: %v", err)
		return
	}

	if err := validateProject(&project); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Project validation failed: %v", err)
		return
	}

	createdProject, err := h.projectService.CreateProject(&project)
	if err != nil {
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		log.Printf("Error creating project: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProject)
}

func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	project, err := h.projectService.GetProject(projectID)
	if err != nil {
		if err == services.ErrProjectNotFound {
			http.Error(w, "Project not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get project", http.StatusInternalServerError)
			log.Printf("Error getting project: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var updatedProject models.Project
	err = json.NewDecoder(r.Body).Decode(&updatedProject)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding updated project: %v", err)
		return
	}

	if err := validateProject(&updatedProject); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Updated project validation failed: %v", err)
		return
	}

	project, err := h.projectService.UpdateProject(projectID, &updatedProject)
	if err != nil {
		if err == services.ErrProjectNotFound {
			http.Error(w, "Project not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update project", http.StatusInternalServerError)
			log.Printf("Error updating project: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	confirm := r.URL.Query().Get("confirm")
	if confirm != "true" {
		http.Error(w, "Confirmation required. Add '?confirm=true' to the request URL to confirm deletion.", http.StatusBadRequest)
		return
	}

	err = h.projectService.DeleteProject(projectID)
	if err != nil {
		if err == services.ErrProjectNotFound {
			http.Error(w, "Project not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete project", http.StatusInternalServerError)
			log.Printf("Error deleting project: %v", err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	page, pageSize := getPaginationParams(r)
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	filter := r.URL.Query().Get("filter")

	projects, totalCount, err := h.projectService.ListProjects(page, pageSize, sortBy, sortOrder, filter)
	if err != nil {
		http.Error(w, "Failed to list projects", http.StatusInternalServerError)
		log.Printf("Error listing projects: %v", err)
		return
	}

	response := map[string]interface{}{
		"projects":   projects,
		"totalCount": totalCount,
		"page":       page,
		"pageSize":   pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getProjectIDFromRequest(r *http.Request) (models.ProjectID, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		return 0, models.ErrInvalidProjectID
	}
	if id <= 0 {
		return 0, models.ErrInvalidProjectID
	}
	return models.ProjectID(id), nil
}

func getPaginationParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return page, pageSize
}

func validateProject(project *models.Project) error {
	if project.Name == "" {
		return models.ErrInvalidProjectName
	}
	if project.Description == "" {
		return models.ErrInvalidProjectDescription
	}
	if project.StartDate.IsZero() {
		return models.ErrInvalidProjectStartDate
	}
	if !project.EndDate.IsZero() && project.EndDate.Before(project.StartDate) {
		return models.ErrInvalidProjectEndDate
	}
	if !isValidStatus(project.Status) {
		return models.ErrInvalidProjectStatus
	}
	return nil
}

func isValidStatus(status string) bool {
	validStatuses := []string{"planning", "in_progress", "completed", "on_hold", "cancelled"}
	for _, s := range validStatuses {
		if status == s {
			return true
		}
	}
	return false
}