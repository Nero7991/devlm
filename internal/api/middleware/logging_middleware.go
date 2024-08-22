package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "HTTP: ", log.Ldate|log.Ltime|log.Lshortfile)
}

type LogConfig struct {
	ExcludeHeaders []string
	ExcludeBodies  []string
	BodySizeLimit  int
	LogFormat      string
	AdditionalFields map[string]string
}

var defaultLogConfig = LogConfig{
	ExcludeHeaders: []string{"Authorization", "Cookie"},
	ExcludeBodies:  []string{"/login", "/register"},
	BodySizeLimit:  1024 * 1024, // 1MB
	LogFormat:      "json",
	AdditionalFields: map[string]string{},
}

func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return LoggingMiddlewareWithConfig(next, defaultLogConfig)
}

func LoggingMiddlewareWithConfig(next http.HandlerFunc, config LogConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrappedWriter := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		var requestBody []byte
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			if !contains(config.ExcludeBodies, r.URL.Path) {
				body, _ := io.ReadAll(io.LimitReader(r.Body, int64(config.BodySizeLimit)))
				requestBody = body
				r.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		}

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start)

		logEntry := map[string]interface{}{
			"method":     r.Method,
			"uri":        r.RequestURI,
			"remoteAddr": r.RemoteAddr,
			"statusCode": wrappedWriter.statusCode,
			"duration":   duration.String(),
			"userAgent":  r.UserAgent(),
		}

		headers := make(map[string]string)
		for k, v := range r.Header {
			if !contains(config.ExcludeHeaders, k) {
				headers[k] = v[0]
			}
		}
		logEntry["headers"] = headers

		if len(requestBody) > 0 {
			logEntry["requestBody"] = string(requestBody)
		}

		for k, v := range config.AdditionalFields {
			logEntry[k] = v
		}

		var logOutput string
		switch strings.ToLower(config.LogFormat) {
		case "json":
			logJSON, _ := json.Marshal(logEntry)
			logOutput = string(logJSON)
		default:
			logOutput = formatLogEntry(logEntry)
		}

		logger.Println(logOutput)
	}
}

func formatLogEntry(entry map[string]interface{}) string {
	var parts []string
	for k, v := range entry {
		parts = append(parts, k+"="+fmt.Sprint(v))
	}
	return strings.Join(parts, " ")
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func ErrorLoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := make([]byte, 4096)
				stack = stack[:runtime.Stack(stack, false)]
				logger.Printf("Panic: %v\nStack:\n%s", err, stack)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	}
}

func SetLogOutput(w io.Writer) {
	if w == nil {
		return
	}
	logger.SetOutput(w)
}

func SetLogFlags(flags int) {
	logger.SetFlags(flags)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}