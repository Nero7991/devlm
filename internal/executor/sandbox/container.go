package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Container represents a sandbox container
type Container struct {
	ID     string
	Name   string
	Client *client.Client
}

// NewContainer creates a new Container instance
func NewContainer(name string, options ...func(*client.Client) error) (*Container, error) {
	opts := []client.Opt{client.FromEnv}
	for _, opt := range options {
		opts = append(opts, opt)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Container{
		Name:   name,
		Client: cli,
	}, nil
}

// Start creates and starts the container
func (c *Container) Start(ctx context.Context) error {
	resp, err := c.Client.ContainerCreate(ctx, &container.Config{
		Image: "ubuntu:latest",
		Cmd:   []string{"/bin/bash"},
		Tty:   true,
	}, &container.HostConfig{
		Resources: container.Resources{
			Memory:    256 * 1024 * 1024, // 256 MB
			CPUPeriod: 100000,
			CPUQuota:  50000,
			PidsLimit: 100,
			IOMaximumBandwidth: 1024 * 1024 * 10, // 10 MB/s
		},
		SecurityOpt: []string{"no-new-privileges"},
		CapDrop:     []string{"ALL"},
		NetworkMode: "none",
	}, nil, nil, c.Name)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	c.ID = resp.ID

	if err := c.Client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// Stop stops and removes the container
func (c *Container) Stop(ctx context.Context) error {
	timeout := int(30)
	if err := c.Client.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	if err := c.Client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// ExecuteCommand runs a command in the container and returns the output
func (c *Container) ExecuteCommand(ctx context.Context, cmd []string) (string, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := c.Client.ContainerExecCreate(ctx, c.ID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := c.Client.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	var outBuf, errBuf io.Writer
	outputChan := make(chan string)
	errChan := make(chan error)

	go func() {
		outBuf = new(bytes.Buffer)
		errBuf = new(bytes.Buffer)
		_, err := stdcopy.StdCopy(outBuf, errBuf, resp.Reader)
		if err != nil {
			errChan <- fmt.Errorf("failed to read exec output: %w", err)
			return
		}
		outputChan <- outBuf.(*bytes.Buffer).String() + errBuf.(*bytes.Buffer).String()
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("command execution timed out")
	case err := <-errChan:
		return "", err
	case output := <-outputChan:
		return output, nil
	}
}

// CopyToContainer copies a file or directory to the container
func (c *Container) CopyToContainer(ctx context.Context, hostPath, containerPath string) error {
	srcInfo, err := os.Stat(hostPath)
	if err != nil {
		return fmt.Errorf("failed to get host file info: %w", err)
	}

	srcFile, err := os.Open(hostPath)
	if err != nil {
		return fmt.Errorf("failed to open host file: %w", err)
	}
	defer srcFile.Close()

	err = c.Client.CopyToContainer(ctx, c.ID, containerPath, srcFile, types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
		CopyUIDGID:                true,
	})
	if err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	if srcInfo.IsDir() {
		_, err = c.ExecuteCommand(ctx, []string{"chmod", "-R", "755", containerPath})
		if err != nil {
			return fmt.Errorf("failed to set permissions on copied directory: %w", err)
		}
	}

	return nil
}

// CopyFromContainer copies a file or directory from the container to the host
func (c *Container) CopyFromContainer(ctx context.Context, containerPath, hostPath string) error {
	reader, _, err := c.Client.CopyFromContainer(ctx, c.ID, containerPath)
	if err != nil {
		return fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	destFile, err := os.Create(hostPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, reader)
	if err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// GetLogs retrieves the logs from the container
func (c *Container) GetLogs(ctx context.Context) (string, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     false,
		Tail:       "all",
	}

	logs, err := c.Client.ContainerLogs(ctx, c.ID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logs.Close()

	logContent, err := io.ReadAll(logs)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	return string(logContent), nil
}

// GetStatus returns the current status of the container
func (c *Container) GetStatus(ctx context.Context) (string, error) {
	inspect, err := c.Client.ContainerInspect(ctx, c.ID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}
	return inspect.State.Status, nil
}

// Restart restarts the container
func (c *Container) Restart(ctx context.Context) error {
	timeout := int(30)
	if err := c.Client.ContainerRestart(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	return nil
}

// Pause pauses the container
func (c *Container) Pause(ctx context.Context) error {
	if err := c.Client.ContainerPause(ctx, c.ID); err != nil {
		return fmt.Errorf("failed to pause container: %w", err)
	}
	return nil
}

// Unpause unpauses the container
func (c *Container) Unpause(ctx context.Context) error {
	if err := c.Client.ContainerUnpause(ctx, c.ID); err != nil {
		return fmt.Errorf("failed to unpause container: %w", err)
	}
	return nil
}

// UpdateResourceLimits updates the resource limits for the container
func (c *Container) UpdateResourceLimits(ctx context.Context, memory int64, cpuQuota int64) error {
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			Memory:    memory,
			CPUQuota:  cpuQuota,
			CPUPeriod: 100000,
			PidsLimit: 100,
			IOMaximumBandwidth: 1024 * 1024 * 10, // 10 MB/s
		},
	}

	_, err := c.Client.ContainerUpdate(ctx, c.ID, updateConfig)
	if err != nil {
		return fmt.Errorf("failed to update container resources: %w", err)
	}

	return nil
}

// GetStats returns the current resource usage statistics of the container
func (c *Container) GetStats(ctx context.Context) (*types.StatsJSON, error) {
	stats, err := c.Client.ContainerStats(ctx, c.ID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer stats.Body.Close()

	var statsJSON types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&statsJSON); err != nil {
		return nil, fmt.Errorf("failed to decode container stats: %w", err)
	}

	return &statsJSON, nil
}