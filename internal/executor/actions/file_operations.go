package actions

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileOperations interface defines methods for file-related operations
type FileOperations interface {
	ReadFile(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, path string, content string, perm os.FileMode) error
	DeleteFile(ctx context.Context, path string) error
	ListDirectory(ctx context.Context, path string, includeSubdirs bool) ([]string, error)
}

// FileManager implements FileOperations interface
type FileManager struct {
	fileLocks sync.Map
}

// NewFileManager creates and returns a new instance of FileManager
func NewFileManager() *FileManager {
	return &FileManager{}
}

// ReadFile reads the content of a file at the given path
func (fm *FileManager) ReadFile(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", os.ErrNotExist
	}

	lock := fm.getOrCreateLock(path)
	lock.Lock()
	defer lock.Unlock()

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// WriteFile writes content to a file at the given path, creating directories if necessary
func (fm *FileManager) WriteFile(ctx context.Context, path string, content string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// Check available disk space before writing
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if fsStat, ok := info.Sys().(*fs.StatVFS); ok {
		availableSpace := fsStat.Bavail * uint64(fsStat.Bsize)
		if uint64(len(content)) > availableSpace {
			return os.ErrInsufficientSpace
		}
	}

	lock := fm.getOrCreateLock(path)
	lock.Lock()
	defer lock.Unlock()

	tempFile := path + ".tmp"
	if err := ioutil.WriteFile(tempFile, []byte(content), perm); err != nil {
		return err
	}

	return os.Rename(tempFile, path)
}

// DeleteFile deletes a file at the given path
func (fm *FileManager) DeleteFile(ctx context.Context, path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return os.ErrInvalid
	}

	if !fm.confirmDeletion(path) {
		return fmt.Errorf("deletion cancelled by user")
	}

	lock := fm.getOrCreateLock(path)
	lock.Lock()
	defer lock.Unlock()

	return os.Remove(path)
}

// ListDirectory lists the contents of a directory at the given path
func (fm *FileManager) ListDirectory(ctx context.Context, path string, includeSubdirs bool) ([]string, error) {
	var fileList []string

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filePath == path {
			return nil
		}
		if !includeSubdirs && info.IsDir() {
			return filepath.SkipDir
		}
		relPath, err := filepath.Rel(path, filePath)
		if err != nil {
			return err
		}
		fileList = append(fileList, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(fileList)

	return fileList, nil
}

func (fm *FileManager) getOrCreateLock(path string) *sync.Mutex {
	lock, _ := fm.fileLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (fm *FileManager) confirmDeletion(path string) bool {
	// TODO: Implement actual confirmation mechanism
	// For now, we'll simulate a confirmation delay
	time.Sleep(time.Second)
	return true
}