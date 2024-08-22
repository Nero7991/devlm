package utils

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ReadFile(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return "", fmt.Errorf("file not found: %v", err)
		case os.IsPermission(err):
			return "", fmt.Errorf("permission denied: %v", err)
		default:
			return "", fmt.Errorf("error reading file: %v", err)
		}
	}
	return string(content), nil
}

func WriteFile(path string, content string, perm os.FileMode) error {
	if perm == 0 {
		perm = 0644
	}
	return ioutil.WriteFile(path, []byte(content), perm)
}

func AppendToFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

func EnsureDirectoryExists(path string, perm os.FileMode) error {
	if perm == 0 {
		perm = os.ModePerm
	}
	return os.MkdirAll(path, perm)
}

func ListFiles(dir string, filter func(string) bool, recursive bool) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filter == nil || filter(path)) {
			files = append(files, path)
		}
		if !recursive && info.IsDir() && path != dir {
			return filepath.SkipDir
		}
		return nil
	})
	return files, err
}

func JSONToStruct(jsonStr string, v interface{}) error {
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON structure: %v", err)
	}
	return nil
}

func StructToJSON(v interface{}, pretty bool) (string, error) {
	var (
		jsonBytes []byte
		err       error
	)
	if pretty {
		jsonBytes, err = json.MarshalIndent(v, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(v)
	}
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func SplitString(s, delimiter string, keepEmpty bool) []string {
	parts := strings.Split(s, delimiter)
	if !keepEmpty {
		var result []string
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, uuid)
	if err != nil {
		return ""
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func GetCurrentTimestamp(format string, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	if format == "" {
		return fmt.Sprintf("%d", now.Unix())
	}
	return now.Format(format)
}

func RandomString(n int, charset string) string {
	if charset == "" {
		charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}
	b := make([]byte, n)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

func IsFileExists(filename string) (bool, os.FileMode, error) {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	return !info.IsDir(), info.Mode().Perm(), nil
}

func CopyFile(src, dst string, preserveMetadata bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory, use CopyDir instead")
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	if preserveMetadata {
		err = os.Chmod(dst, srcInfo.Mode())
		if err != nil {
			return err
		}

		err = os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
		if err != nil {
			return err
		}
	}

	return nil
}

func CopyDir(src, dst string, preserveMetadata bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath, preserveMetadata)
			if err != nil {
				return err
			}
		} else {
			err = CopyFile(srcPath, dstPath, preserveMetadata)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ReadLargeFile(path string, chunkSize int, processChunk func([]byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	buffer := make([]byte, chunkSize)

	for {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 {
			break
		}

		if err := processChunk(buffer[:n]); err != nil {
			return err
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}