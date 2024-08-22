package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// SandboxManager manages the creation, execution, and cleanup of sandbox containers
type SandboxManager struct {
	containers   map[string]*Container
	mu           sync.Mutex
	dockerClient *client.Client
}

// NewSandboxManager creates a new SandboxManager instance
func NewSandboxManager(opts ...func(*client.Client) error) (*SandboxManager, error) {
	cli, err := client.NewClientWithOpts(append([]func(*client.Client) error{client.FromEnv}, opts...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &SandboxManager{
		containers:   make(map[string]*Container),
		dockerClient: cli,
	}, nil
}

// CreateContainer creates a new sandbox container
func (sm *SandboxManager) CreateContainer(ctx context.Context, name string, config *container.Config, hostConfig *container.HostConfig) (*Container, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if config == nil {
		config = &container.Config{
			Image: "golang:latest",
			Cmd:   []string{"/bin/bash"},
			Tty:   true,
		}
	}

	if hostConfig == nil {
		hostConfig = &container.HostConfig{
			AutoRemove: true,
			Privileged: false,
			Resources: container.Resources{
				Memory:    256 * 1024 * 1024, // 256MB
				CPUPeriod: 100000,
				CPUQuota:  50000, // 0.5 CPU
			},
			SecurityOpt: []string{"no-new-privileges"},
			NetworkMode: "none",
		}
	}

	resp, err := sm.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := sm.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	container := &Container{
		ID:   resp.ID,
		Name: name,
	}

	sm.containers[name] = container
	return container, nil
}

// ExecuteCode runs the provided code in the specified container
func (sm *SandboxManager) ExecuteCode(ctx context.Context, containerName, code string, timeout time.Duration) (string, error) {
	sm.mu.Lock()
	container, exists := sm.containers[containerName]
	sm.mu.Unlock()

	if !exists {
		return "", fmt.Errorf("container %s not found", containerName)
	}

	tempDir, err := os.MkdirTemp("", "sandbox")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write code to file: %w", err)
	}

	if err := sm.copyToContainer(ctx, container.ID, filePath, "/main.go"); err != nil {
		return "", fmt.Errorf("failed to copy code to container: %w", err)
	}

	execConfig := types.ExecConfig{
		Cmd:          []string{"go", "run", "/main.go"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := sm.dockerClient.ContainerExecCreate(ctx, container.ID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := sm.dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	outputChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		var output strings.Builder
		_, err := stdcopy.StdCopy(&output, &output, resp.Reader)
		if err != nil {
			errorChan <- fmt.Errorf("failed to read exec output: %w", err)
		} else {
			outputChan <- output.String()
		}
	}()

	select {
	case output := <-outputChan:
		return output, nil
	case err := <-errorChan:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("execution timed out after %v", timeout)
	}
}

// CleanupContainer stops and removes the specified container
func (sm *SandboxManager) CleanupContainer(ctx context.Context, name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	container, exists := sm.containers[name]
	if !exists {
		return fmt.Errorf("container %s not found", name)
	}

	timeout := 10 * time.Second
	if err := sm.dockerClient.ContainerStop(ctx, container.ID, &timeout); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	if err := sm.dockerClient.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	delete(sm.containers, name)
	return nil
}

// ListContainers returns a list of all active containers
func (sm *SandboxManager) ListContainers() []*Container {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	containers := make([]*Container, 0, len(sm.containers))
	for _, container := range sm.containers {
		containers = append(containers, container)
	}
	return containers
}

func (sm *SandboxManager) copyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	stat, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stats: %w", err)
	}

	return sm.dockerClient.CopyToContainer(ctx, containerID, filepath.Dir(dstPath), srcFile, types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
		CopyUIDGID:                true,
	})
}

// SetResourceLimits updates the resource limits for a container
func (sm *SandboxManager) SetResourceLimits(ctx context.Context, containerName string, memory int64, cpuQuota int64) error {
	sm.mu.Lock()
	container, exists := sm.containers[containerName]
	sm.mu.Unlock()

	if !exists {
		return fmt.Errorf("container %s not found", containerName)
	}

	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			Memory:    memory,
			CPUQuota:  cpuQuota,
			CPUPeriod: 100000,
		},
	}

	_, err := sm.dockerClient.ContainerUpdate(ctx, container.ID, updateConfig)
	if err != nil {
		return fmt.Errorf("failed to update container resource limits: %w", err)
	}

	return nil
}

// GetContainerLogs retrieves the logs from a container
func (sm *SandboxManager) GetContainerLogs(ctx context.Context, containerName string) (string, error) {
	sm.mu.Lock()
	container, exists := sm.containers[containerName]
	sm.mu.Unlock()

	if !exists {
		return "", fmt.Errorf("container %s not found", containerName)
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Since:      time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	}

	reader, err := sm.dockerClient.ContainerLogs(ctx, container.ID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	var logs strings.Builder
	_, err = io.Copy(&logs, reader)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	return logs.String(), nil
}

// Close closes the Docker client connection
func (sm *SandboxManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, container := range sm.containers {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := sm.CleanupContainer(ctx, container.Name); err != nil {
			cancel()
			return fmt.Errorf("failed to cleanup container %s: %w", container.Name, err)
		}
		cancel()
	}

	return sm.dockerClient.Close()
}