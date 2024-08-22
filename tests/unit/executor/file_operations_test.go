package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileOperations_ReadFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")
	content := []byte("Hello, World!")
	err := ioutil.WriteFile(tempFile, content, 0644)
	assert.NoError(t, err)

	fo := NewFileOperations()

	readContent, err := fo.ReadFile(tempFile)
	assert.NoError(t, err)
	assert.Equal(t, content, readContent)

	_, err = fo.ReadFile("non_existent_file.txt")
	assert.Error(t, err)

	_, err = fo.ReadFile(filepath.Join(tempDir, "not_found.txt"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	largeTempFile := filepath.Join(tempDir, "large_test.txt")
	largeContent := make([]byte, 10*1024*1024) // 10 MB
	err = ioutil.WriteFile(largeTempFile, largeContent, 0644)
	assert.NoError(t, err)

	readLargeContent, err := fo.ReadFile(largeTempFile)
	assert.NoError(t, err)
	assert.Equal(t, largeContent, readLargeContent)

	restrictedFile := filepath.Join(tempDir, "restricted.txt")
	err = ioutil.WriteFile(restrictedFile, []byte("Restricted content"), 0400)
	assert.NoError(t, err)

	readRestrictedContent, err := fo.ReadFile(restrictedFile)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Restricted content"), readRestrictedContent)

	utf8File := filepath.Join(tempDir, "utf8.txt")
	utf8Content := []byte("こんにちは世界")
	err = ioutil.WriteFile(utf8File, utf8Content, 0644)
	assert.NoError(t, err)

	readUTF8Content, err := fo.ReadFile(utf8File)
	assert.NoError(t, err)
	assert.Equal(t, utf8Content, readUTF8Content)

	specialCharsFile := filepath.Join(tempDir, "special!@#$%^&*()_+.txt")
	specialContent := []byte("Special content")
	err = ioutil.WriteFile(specialCharsFile, specialContent, 0644)
	assert.NoError(t, err)

	readSpecialContent, err := fo.ReadFile(specialCharsFile)
	assert.NoError(t, err)
	assert.Equal(t, specialContent, readSpecialContent)

	longPath := filepath.Join(tempDir, "a", "very", "long", "path", "to", "a", "file.txt")
	err = os.MkdirAll(filepath.Dir(longPath), 0755)
	assert.NoError(t, err)
	err = ioutil.WriteFile(longPath, []byte("Long path content"), 0644)
	assert.NoError(t, err)

	readLongPathContent, err := fo.ReadFile(longPath)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Long path content"), readLongPathContent)

	// Test concurrent file reading
	concurrentFiles := 100
	errorChan := make(chan error, concurrentFiles)
	for i := 0; i < concurrentFiles; i++ {
		go func(index int) {
			_, err := fo.ReadFile(tempFile)
			errorChan <- err
		}(i)
	}
	for i := 0; i < concurrentFiles; i++ {
		err := <-errorChan
		assert.NoError(t, err)
	}
}

func TestFileOperations_WriteFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")

	fo := NewFileOperations()

	content := []byte("Hello, World!")
	err := fo.WriteFile(tempFile, content)
	assert.NoError(t, err)

	readContent, err := ioutil.ReadFile(tempFile)
	assert.NoError(t, err)
	assert.Equal(t, content, readContent)

	readOnlyDir := filepath.Join(tempDir, "readonly")
	err = os.Mkdir(readOnlyDir, 0444)
	assert.NoError(t, err)
	defer os.Chmod(readOnlyDir, 0755)

	readOnlyFile := filepath.Join(readOnlyDir, "test.txt")
	err = fo.WriteFile(readOnlyFile, content)
	assert.Error(t, err)

	readOnlyFile = filepath.Join(tempDir, "readonly.txt")
	err = ioutil.WriteFile(readOnlyFile, []byte("initial content"), 0444)
	assert.NoError(t, err)
	err = fo.WriteFile(readOnlyFile, content)
	assert.Error(t, err)

	existingFile := filepath.Join(tempDir, "existing.txt")
	err = ioutil.WriteFile(existingFile, []byte("Initial content"), 0644)
	assert.NoError(t, err)

	newContent := []byte("Updated content")
	err = fo.WriteFile(existingFile, newContent)
	assert.NoError(t, err)

	readUpdatedContent, err := ioutil.ReadFile(existingFile)
	assert.NoError(t, err)
	assert.Equal(t, newContent, readUpdatedContent)

	largeContent := make([]byte, 50*1024*1024) // 50 MB
	largeTempFile := filepath.Join(tempDir, "large_test.txt")
	err = fo.WriteFile(largeTempFile, largeContent)
	assert.NoError(t, err)

	readLargeContent, err := ioutil.ReadFile(largeTempFile)
	assert.NoError(t, err)
	assert.Equal(t, largeContent, readLargeContent)

	utf8File := filepath.Join(tempDir, "utf8.txt")
	utf8Content := []byte("こんにちは世界")
	err = fo.WriteFile(utf8File, utf8Content)
	assert.NoError(t, err)

	readUTF8Content, err := ioutil.ReadFile(utf8File)
	assert.NoError(t, err)
	assert.Equal(t, utf8Content, readUTF8Content)

	nonExistentDir := filepath.Join(tempDir, "non_existent_dir")
	nonExistentFile := filepath.Join(nonExistentDir, "test.txt")
	err = fo.WriteFile(nonExistentFile, content)
	assert.Error(t, err)

	specialCharsFile := filepath.Join(tempDir, "special!@#$%^&*()_+.txt")
	specialContent := []byte("Special content")
	err = fo.WriteFile(specialCharsFile, specialContent)
	assert.NoError(t, err)

	readSpecialContent, err := ioutil.ReadFile(specialCharsFile)
	assert.NoError(t, err)
	assert.Equal(t, specialContent, readSpecialContent)

	longPath := filepath.Join(tempDir, "a", "very", "long", "path", "to", "a", "file.txt")
	err = os.MkdirAll(filepath.Dir(longPath), 0755)
	assert.NoError(t, err)
	err = fo.WriteFile(longPath, []byte("Long path content"))
	assert.NoError(t, err)

	readLongPathContent, err := ioutil.ReadFile(longPath)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Long path content"), readLongPathContent)

	// Test concurrent file writing
	concurrentFiles := 100
	errorChan := make(chan error, concurrentFiles)
	for i := 0; i < concurrentFiles; i++ {
		go func(index int) {
			filename := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			err := fo.WriteFile(filename, []byte(fmt.Sprintf("Content %d", index)))
			errorChan <- err
		}(i)
	}
	for i := 0; i < concurrentFiles; i++ {
		err := <-errorChan
		assert.NoError(t, err)
	}

	// Test writing to a full disk (simulated)
	fullDiskDir := filepath.Join(tempDir, "full_disk")
	err = os.Mkdir(fullDiskDir, 0755)
	assert.NoError(t, err)
	// Simulate a full disk by making the directory read-only
	err = os.Chmod(fullDiskDir, 0444)
	assert.NoError(t, err)
	defer os.Chmod(fullDiskDir, 0755)

	fullDiskFile := filepath.Join(fullDiskDir, "test.txt")
	err = fo.WriteFile(fullDiskFile, []byte("Test content"))
	assert.Error(t, err)
}

func TestFileOperations_ListFiles(t *testing.T) {
	tempDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	assert.NoError(t, err)

	files := []string{"file1.txt", "file2.txt", filepath.Join("subdir", "file3.txt")}
	for _, file := range files {
		err := ioutil.WriteFile(filepath.Join(tempDir, file), []byte("content"), 0644)
		assert.NoError(t, err)
	}

	fo := NewFileOperations()

	listedFiles, err := fo.ListFiles(tempDir)
	assert.NoError(t, err)
	assert.ElementsMatch(t, files, listedFiles)

	_, err = fo.ListFiles("non_existent_dir")
	assert.Error(t, err)

	emptyDir := filepath.Join(tempDir, "empty")
	err = os.Mkdir(emptyDir, 0755)
	assert.NoError(t, err)
	emptyFiles, err := fo.ListFiles(emptyDir)
	assert.NoError(t, err)
	assert.Empty(t, emptyFiles)

	largeDir := filepath.Join(tempDir, "large_dir")
	err = os.Mkdir(largeDir, 0755)
	assert.NoError(t, err)

	largeFileCount := 10000
	for i := 0; i < largeFileCount; i++ {
		filename := filepath.Join(largeDir, fmt.Sprintf("file_%d.txt", i))
		err := ioutil.WriteFile(filename, []byte("content"), 0644)
		assert.NoError(t, err)
	}

	largeListStart := time.Now()
	largeListedFiles, err := fo.ListFiles(largeDir)
	largeListDuration := time.Since(largeListStart)
	assert.NoError(t, err)
	assert.Len(t, largeListedFiles, largeFileCount)
	assert.Less(t, largeListDuration, 5*time.Second)

	symlinkDir := filepath.Join(tempDir, "symlink_dir")
	err = os.Mkdir(symlinkDir, 0755)
	assert.NoError(t, err)

	symlinkTarget := filepath.Join(symlinkDir, "target.txt")
	err = ioutil.WriteFile(symlinkTarget, []byte("symlink target"), 0644)
	assert.NoError(t, err)

	symlinkSource := filepath.Join(symlinkDir, "source.txt")
	err = os.Symlink(symlinkTarget, symlinkSource)
	assert.NoError(t, err)

	symlinkFiles, err := fo.ListFiles(symlinkDir)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"target.txt", "source.txt"}, symlinkFiles)

	specialCharsDir := filepath.Join(tempDir, "special_chars")
	err = os.Mkdir(specialCharsDir, 0755)
	assert.NoError(t, err)

	specialFiles := []string{"file!.txt", "file@.txt", "file#.txt", "file$.txt", "file%.txt"}
	for _, file := range specialFiles {
		err := ioutil.WriteFile(filepath.Join(specialCharsDir, file), []byte("content"), 0644)
		assert.NoError(t, err)
	}

	listedSpecialFiles, err := fo.ListFiles(specialCharsDir)
	assert.NoError(t, err)
	assert.ElementsMatch(t, specialFiles, listedSpecialFiles)

	longPathDir := filepath.Join(tempDir, "a", "very", "long", "path", "to", "a", "directory")
	err = os.MkdirAll(longPathDir, 0755)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(longPathDir, "file.txt"), []byte("content"), 0644)
	assert.NoError(t, err)

	longPathFiles, err := fo.ListFiles(longPathDir)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"file.txt"}, longPathFiles)

	// Test listing files with different sorting options
	sortedDir := filepath.Join(tempDir, "sorted_dir")
	err = os.Mkdir(sortedDir, 0755)
	assert.NoError(t, err)

	sortedFiles := []string{"b.txt", "a.txt", "c.txt"}
	for _, file := range sortedFiles {
		err := ioutil.WriteFile(filepath.Join(sortedDir, file), []byte("content"), 0644)
		assert.NoError(t, err)
	}

	// Test sorting by name (ascending)
	sortedByNameAsc, err := fo.ListFilesSorted(sortedDir, "name", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a.txt", "b.txt", "c.txt"}, sortedByNameAsc)

	// Test sorting by name (descending)
	sortedByNameDesc, err := fo.ListFilesSorted(sortedDir, "name", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"c.txt", "b.txt", "a.txt"}, sortedByNameDesc)

	// Test filtering files
	filteredFiles, err := fo.ListFilesFiltered(tempDir, ".txt")
	assert.NoError(t, err)
	for _, file := range filteredFiles {
		assert.True(t, filepath.Ext(file) == ".txt")
	}

	// Test recursive file listing
	recursiveDir := filepath.Join(tempDir, "recursive")
	err = os.MkdirAll(filepath.Join(recursiveDir, "subdir1", "subdir2"), 0755)
	assert.NoError(t, err)
	recursiveFiles := []string{
		"file1.txt",
		filepath.Join("subdir1", "file2.txt"),
		filepath.Join("subdir1", "subdir2", "file3.txt"),
	}