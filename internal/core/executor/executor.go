package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Executor struct {
	dockerClient *client.Client
	fileLocks    sync.Map
	searchCache  sync.Map
}

func NewExecutor(opts ...client.Opt) (*Executor, error) {
	cli, err := client.NewClientWithOpts(append([]client.Opt{client.FromEnv}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &Executor{
		dockerClient: cli,
		fileLocks:    sync.Map{},
		searchCache:  sync.Map{},
	}, nil
}

func (e *Executor) ExecuteCode(ctx context.Context, code string, language string) (string, error) {
	switch language {
	case "go":
		return e.executeInDocker(ctx, code, "golang:latest", "go", "run", "/app/main.go")
	case "python":
		return e.executeInDocker(ctx, code, "python:latest", "python", "/app/script.py")
	case "javascript":
		return e.executeInDocker(ctx, code, "node:latest", "node", "/app/script.js")
	case "ruby":
		return e.executeInDocker(ctx, code, "ruby:latest", "ruby", "/app/script.rb")
	case "rust":
		return e.executeInDocker(ctx, code, "rust:latest", "sh", "-c", "rustc -o /app/output /app/main.rs && /app/output")
	case "java":
		return e.executeInDocker(ctx, code, "openjdk:latest", "sh", "-c", "javac /app/Main.java && java -cp /app Main")
	case "cpp":
		return e.executeInDocker(ctx, code, "gcc:latest", "sh", "-c", "g++ -o /app/output /app/main.cpp && /app/output")
	case "php":
		return e.executeInDocker(ctx, code, "php:latest", "php", "/app/script.php")
	case "csharp":
		return e.executeInDocker(ctx, code, "mcr.microsoft.com/dotnet/sdk:latest", "sh", "-c", "dotnet new console -o /app && echo \"$1\" > /app/Program.cs && dotnet run --project /app")
	case "swift":
		return e.executeInDocker(ctx, code, "swift:latest", "sh", "-c", "echo \"$1\" > /app/main.swift && swift /app/main.swift")
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func (e *Executor) executeInDocker(ctx context.Context, code, image string, command ...string) (string, error) {
	resp, err := e.dockerClient.ContainerCreate(ctx, &container.Config{
		Image:      image,
		Cmd:        command,
		WorkingDir: "/app",
	}, &container.HostConfig{
		AutoRemove: true,
		Resources: container.Resources{
			Memory:    256 * 1024 * 1024,
			CPUPeriod: 100000,
			CPUQuota:  50000,
		},
	}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	defer func() {
		timeout := 5 * time.Second
		err := e.dockerClient.ContainerStop(ctx, resp.ID, &timeout)
		if err != nil {
			fmt.Printf("failed to stop container: %v\n", err)
		}
		if err := e.dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
			fmt.Printf("failed to remove container: %v\n", err)
		}
	}()

	err = e.dockerClient.CopyToContainer(ctx, resp.ID, "/app", strings.NewReader(code), types.CopyToContainerOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to copy code to container: %w", err)
	}

	if err := e.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	statusCh, errCh := e.dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("error waiting for container: %w", err)
		}
	case <-statusCh:
	}

	out, err := e.dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", fmt.Errorf("error getting container logs: %w", err)
	}
	defer out.Close()

	var logs strings.Builder
	_, err = io.Copy(&logs, out)
	if err != nil {
		return "", fmt.Errorf("error reading container logs: %w", err)
	}

	return logs.String(), nil
}

func (e *Executor) ReadFile(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var content strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		if n == 0 {
			break
		}
		content.Write(buf[:n])
	}
	return content.String(), nil
}

func (e *Executor) WriteFile(ctx context.Context, path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	lock, _ := e.fileLocks.LoadOrStore(path, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	tempFile := path + ".tmp"
	file, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temporary file for writing: %w", err)
	}

	_, err = io.WriteString(file, content)
	if err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

func (e *Executor) PerformWebSearch(ctx context.Context, query string) (string, error) {
	cacheKey := query
	if cachedResult, ok := e.searchCache.Load(cacheKey); ok {
		return cachedResult.(string), nil
	}

	apiKey := os.Getenv("SEARCH_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("SEARCH_API_KEY environment variable not set")
	}

	url := fmt.Sprintf("https://api.search.example.com/v1/search?q=%s&api_key=%s", query, apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform web search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search API returned non-OK status: %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"url"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	var sb strings.Builder
	for _, r := range result.Results {
		sb.WriteString(fmt.Sprintf("Title: %s\nDescription: %s\nURL: %s\n\n", r.Title, r.Description, r.URL))
	}

	searchResult := sb.String()
	e.searchCache.Store(cacheKey, searchResult)

	return searchResult, nil
}