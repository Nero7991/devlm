package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ActionExecutor struct {
	searchAPIBaseURL string
	fileManager      *FileManager
	webSearch        *WebSearch
}

func NewActionExecutor(searchAPIBaseURL string) *ActionExecutor {
	if searchAPIBaseURL == "" {
		searchAPIBaseURL = "https://api.example.com/search" // Default placeholder URL
	}

	return &ActionExecutor{
		searchAPIBaseURL: searchAPIBaseURL,
		fileManager:      NewFileManager(),
		webSearch:        NewWebSearch(NewGoogleSearcher("YOUR_API_KEY", "YOUR_SEARCH_ENGINE_ID")),
	}
}

func (ae *ActionExecutor) ReadFile(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}
	return ae.fileManager.ReadFile(ctx, path)
}

func (ae *ActionExecutor) WriteFile(ctx context.Context, path string, content string, perm os.FileMode) error {
	// Check for available disk space before writing
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("error checking directory: %w", err)
	}
	if info.Free() < uint64(len(content)) {
		return errors.New("insufficient disk space")
	}
	return ae.fileManager.WriteFile(ctx, path, content, perm)
}

func (ae *ActionExecutor) DeleteFile(ctx context.Context, path string) error {
	// Implement confirmation mechanism
	if !ae.confirmDeletion(path) {
		return errors.New("deletion cancelled by user")
	}
	return ae.fileManager.DeleteFile(ctx, path)
}

func (ae *ActionExecutor) ListDirectory(ctx context.Context, path string, includeSubdirs bool) ([]string, error) {
	files, err := ae.fileManager.ListDirectory(ctx, path, includeSubdirs)
	if err != nil {
		return nil, err
	}
	// Sort files alphabetically
	sort.Strings(files)
	return files, nil
}

func (ae *ActionExecutor) WebSearch(ctx context.Context, query string) (string, error) {
	// Implement caching mechanism
	cacheKey := "websearch:" + query
	if cachedResult, err := ae.getFromCache(cacheKey); err == nil {
		return cachedResult, nil
	}
	result, err := ae.webSearch.Search(ctx, query)
	if err == nil {
		ae.setInCache(cacheKey, result, 1*time.Hour)
	}
	return result, err
}

func (ae *ActionExecutor) ExecuteAction(ctx context.Context, actionType string, params map[string]string) (string, error) {
	if err := ae.validateParams(actionType, params); err != nil {
		return "", err
	}

	switch actionType {
	case "read_file":
		return ae.ReadFile(ctx, params["path"])
	case "write_file":
		perm := os.FileMode(0644)
		if permStr, ok := params["perm"]; ok {
			if permInt, err := strconv.ParseInt(permStr, 8, 32); err == nil {
				perm = os.FileMode(permInt)
			}
		}
		return "", ae.WriteFile(ctx, params["path"], params["content"], perm)
	case "delete_file":
		return "", ae.DeleteFile(ctx, params["path"])
	case "list_directory":
		includeSubdirs := params["include_subdirs"] == "true"
		files, err := ae.ListDirectory(ctx, params["path"], includeSubdirs)
		if err != nil {
			return "", err
		}
		return strings.Join(files, "\n"), nil
	case "web_search":
		return ae.WebSearch(ctx, params["query"])
	default:
		return "", fmt.Errorf("unknown action type: %s", actionType)
	}
}

func (ae *ActionExecutor) validateParams(actionType string, params map[string]string) error {
	requiredParams := map[string][]string{
		"read_file":      {"path"},
		"write_file":     {"path", "content"},
		"delete_file":    {"path"},
		"list_directory": {"path"},
		"web_search":     {"query"},
	}

	if required, ok := requiredParams[actionType]; ok {
		for _, param := range required {
			if _, exists := params[param]; !exists {
				return fmt.Errorf("missing '%s' parameter for %s action", param, actionType)
			}
		}
		return nil
	}
	return fmt.Errorf("unknown action type: %s", actionType)
}

func (ae *ActionExecutor) confirmDeletion(path string) bool {
	// Implement actual confirmation logic here
	return true
}

func (ae *ActionExecutor) getFromCache(key string) (string, error) {
	// Implement actual cache retrieval logic here
	return "", errors.New("not found in cache")
}

func (ae *ActionExecutor) setInCache(key string, value string, expiration time.Duration) {
	// Implement actual cache setting logic here
}