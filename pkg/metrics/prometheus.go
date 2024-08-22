package metrics

import (
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"log"
	"strings"
)

var (
	TotalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_total_requests",
			Help: "The total number of requests processed",
		},
		[]string{"endpoint"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "devlm_request_duration_seconds",
			Help:    "The duration of requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	ErrorCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_error_count",
			Help: "The total number of errors encountered",
		},
		[]string{"type"},
	)

	CodeExecutionCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_code_execution_count",
			Help: "The total number of code executions performed",
		},
		[]string{"type"},
	)

	CodeExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "devlm_code_execution_duration_seconds",
			Help:    "The duration of code executions in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"},
	)

	LLMRequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_llm_request_count",
			Help: "The total number of requests made to the LLM service",
		},
		[]string{"service"},
	)

	LLMRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "devlm_llm_request_duration_seconds",
			Help:    "The duration of LLM requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service"},
	)

	CacheHitCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_cache_hit_count",
			Help: "The total number of cache hits",
		},
		[]string{"cache"},
	)

	CacheMissCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_cache_miss_count",
			Help: "The total number of cache misses",
		},
		[]string{"cache"},
	)

	FileOperationCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_file_operation_count",
			Help: "The total number of file operations performed",
		},
		[]string{"operation"},
	)

	WebSearchCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devlm_web_search_count",
			Help: "The total number of web searches performed",
		},
		[]string{"engine"},
	)
)

var (
	ValidEndpoints = map[string]bool{
		"api":      true,
		"executor": true,
		"llm":      true,
	}

	ValidErrorTypes = map[string]bool{
		"internal":    true,
		"external":    true,
		"validation":  true,
		"permissions": true,
	}

	ValidFileOperations = map[string]bool{
		"read":    true,
		"write":   true,
		"delete":  true,
		"create":  true,
		"update":  true,
		"list":    true,
		"rename":  true,
		"copy":    true,
		"move":    true,
		"compare": true,
	}

	ValidCodeExecutionTypes = map[string]bool{
		"sandbox":   true,
		"container": true,
		"local":     true,
	}

	ValidLLMServices = map[string]bool{
		"claude":    true,
		"gpt3":      true,
		"custom":    true,
		"openai":    true,
		"anthropic": true,
	}

	ValidCacheTypes = map[string]bool{
		"redis":     true,
		"memcached": true,
		"local":     true,
	}

	ValidSearchEngines = map[string]bool{
		"google":     true,
		"bing":       true,
		"duckduckgo": true,
	}
)

func IncrementTotalRequests(endpoint string) error {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	if ValidEndpoints[endpoint] {
		TotalRequests.WithLabelValues(endpoint).Inc()
		return nil
	}
	return errors.New("invalid endpoint")
}

func ObserveRequestDuration(endpoint string, duration float64) error {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	if !ValidEndpoints[endpoint] {
		return errors.New("invalid endpoint")
	}
	if duration < 0 {
		return errors.New("negative duration value")
	}
	RequestDuration.WithLabelValues(endpoint).Observe(duration)
	return nil
}

func IncrementErrorCount(errorType string) error {
	errorType = strings.ToLower(strings.TrimSpace(errorType))
	if ValidErrorTypes[errorType] {
		ErrorCount.WithLabelValues(errorType).Inc()
		return nil
	}
	return errors.New("invalid error type")
}

func IncrementCodeExecutionCount(executionType string) error {
	executionType = strings.ToLower(strings.TrimSpace(executionType))
	if ValidCodeExecutionTypes[executionType] {
		CodeExecutionCount.WithLabelValues(executionType).Inc()
		return nil
	}
	return errors.New("invalid execution type")
}

func ObserveCodeExecutionDuration(executionType string, duration float64) error {
	executionType = strings.ToLower(strings.TrimSpace(executionType))
	if !ValidCodeExecutionTypes[executionType] {
		return errors.New("invalid execution type")
	}
	if duration < 0 {
		return errors.New("negative duration value")
	}
	CodeExecutionDuration.WithLabelValues(executionType).Observe(duration)
	return nil
}

func IncrementLLMRequestCount(service string) error {
	service = strings.ToLower(strings.TrimSpace(service))
	if ValidLLMServices[service] {
		LLMRequestCount.WithLabelValues(service).Inc()
		return nil
	}
	return errors.New("invalid LLM service")
}

func ObserveLLMRequestDuration(service string, duration float64) error {
	service = strings.ToLower(strings.TrimSpace(service))
	if !ValidLLMServices[service] {
		return errors.New("invalid LLM service")
	}
	if duration < 0 {
		return errors.New("negative duration value")
	}
	LLMRequestDuration.WithLabelValues(service).Observe(duration)
	return nil
}

func IncrementCacheHitCount(cacheType string) error {
	cacheType = strings.ToLower(strings.TrimSpace(cacheType))
	if ValidCacheTypes[cacheType] {
		CacheHitCount.WithLabelValues(cacheType).Inc()
		return nil
	}
	return errors.New("invalid cache type")
}

func IncrementCacheMissCount(cacheType string) error {
	cacheType = strings.ToLower(strings.TrimSpace(cacheType))
	if ValidCacheTypes[cacheType] {
		CacheMissCount.WithLabelValues(cacheType).Inc()
		return nil
	}
	return errors.New("invalid cache type")
}

func IncrementFileOperationCount(operation string) error {
	operation = strings.ToLower(strings.TrimSpace(operation))
	if ValidFileOperations[operation] {
		FileOperationCount.WithLabelValues(operation).Inc()
		return nil
	}
	return errors.New("invalid file operation")
}

func IncrementWebSearchCount(engine string) error {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if ValidSearchEngines[engine] {
		WebSearchCount.WithLabelValues(engine).Inc()
		return nil
	}
	return errors.New("invalid search engine")
}