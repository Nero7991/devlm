package logger

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	warnLogger  *log.Logger
	once        sync.Once
	logLevel    LogLevel
	logFile     *os.File
	maxFileSize int64 = 10 * 1024 * 1024 // 10MB
	logFormat   string
	rotateSize  int64
	logFiles    map[string]*os.File
)

type LogConfig struct {
	Level      LogLevel
	FilePath   string
	Format     string
	RotateSize int64
}

func InitLogger(config LogConfig) error {
	var err error
	once.Do(func() {
		logLevel = config.Level
		logFormat = config.Format
		rotateSize = config.RotateSize
		logFiles = make(map[string]*os.File)

		logFile, err = os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return
		}
		logFiles["main"] = logFile

		multiWriter := io.MultiWriter(os.Stdout, logFile)

		flags := log.Ldate | log.Ltime | log.Lshortfile
		if logFormat == "json" {
			flags = 0
		}

		infoLogger = log.New(multiWriter, "", flags)
		errorLogger = log.New(multiWriter, "", flags)
		debugLogger = log.New(multiWriter, "", flags)
		warnLogger = log.New(multiWriter, "", flags)
	})
	return err
}

func logMessage(logger *log.Logger, level LogLevel, format string, v ...interface{}) {
	if logLevel <= level {
		rotateLogFileIfNeeded()
		msg := fmt.Sprintf(format, v...)
		if logFormat == "json" {
			logJSON(logger, level, msg)
		} else {
			logger.Printf("%s: %s", level.String(), msg)
		}
	}
}

func Info(format string, v ...interface{}) {
	logMessage(infoLogger, INFO, format, v...)
}

func Error(format string, v ...interface{}) {
	logMessage(errorLogger, ERROR, format, v...)
}

func Debug(format string, v ...interface{}) {
	logMessage(debugLogger, DEBUG, format, v...)
}

func Warn(format string, v ...interface{}) {
	logMessage(warnLogger, WARN, format, v...)
}

func rotateLogFileIfNeeded() {
	for name, file := range logFiles {
		if file == nil {
			continue
		}

		info, err := file.Stat()
		if err != nil {
			Error("Error getting log file info: %v", err)
			continue
		}

		if info.Size() < rotateSize {
			continue
		}

		file.Close()

		newFileName := fmt.Sprintf("%s.%s.gz", file.Name(), time.Now().Format("2006-01-02-15-04-05"))
		err = compressLogFile(file.Name(), newFileName)
		if err != nil {
			Error("Error compressing log file: %v", err)
			continue
		}

		err = os.Remove(file.Name())
		if err != nil {
			Error("Error removing old log file: %v", err)
		}

		newFile, err := os.OpenFile(file.Name(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			Error("Error creating new log file: %v", err)
			continue
		}

		logFiles[name] = newFile

		multiWriter := io.MultiWriter(os.Stdout, newFile)
		infoLogger.SetOutput(multiWriter)
		errorLogger.SetOutput(multiWriter)
		debugLogger.SetOutput(multiWriter)
		warnLogger.SetOutput(multiWriter)
	}
}

func compressLogFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	gzipWriter := gzip.NewWriter(destFile)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, sourceFile)
	return err
}

type StructuredLog struct {
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Time    time.Time              `json:"time"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

func LogStructured(level LogLevel, message string, data map[string]interface{}) {
	logEntry := StructuredLog{
		Level:   level.String(),
		Message: message,
		Time:    time.Now(),
		Data:    data,
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		Error("Error marshaling structured log: %v", err)
		return
	}

	switch level {
	case DEBUG:
		Debug("%s", string(jsonData))
	case INFO:
		Info("%s", string(jsonData))
	case WARN:
		Warn("%s", string(jsonData))
	case ERROR:
		Error("%s", string(jsonData))
	}
}

func logJSON(logger *log.Logger, level LogLevel, message string) {
	logEntry := StructuredLog{
		Level:   level.String(),
		Message: message,
		Time:    time.Now(),
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		logger.Printf("Error marshaling log entry: %v", err)
		return
	}

	logger.Println(string(jsonData))
}

var logLevelStrings = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

func (l LogLevel) String() string {
	if s, ok := logLevelStrings[l]; ok {
		return s
	}
	return "UNKNOWN"
}

func CleanupOldLogs(logDir string, maxAge time.Duration, filePattern string) error {
	now := time.Now()
	re, err := regexp.Compile(filePattern)
	if err != nil {
		return fmt.Errorf("invalid file pattern: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	err = filepath.Walk(logDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if re.MatchString(info.Name()) && now.Sub(info.ModTime()) > maxAge {
			wg.Add(1)
			go func(filePath string) {
				defer wg.Done()
				if err := os.Remove(filePath); err != nil {
					errChan <- fmt.Errorf("error removing file %s: %v", filePath, err)
				}
			}(path)
		}
		return nil
	})

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return err
}