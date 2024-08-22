package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/api"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestLoadAPI(t *testing.T) {
	apiURL := "http://localhost:8080/api"
	client := &http.Client{Timeout: 10 * time.Second}

	scenarios := []struct {
		name     string
		duration time.Duration
		rps      int
		maxP95   time.Duration
		maxP99   time.Duration
	}{
		{"Light Load", 30 * time.Second, 10, 200 * time.Millisecond, 500 * time.Millisecond},
		{"Medium Load", 60 * time.Second, 50, 300 * time.Millisecond, 750 * time.Millisecond},
		{"Heavy Load", 120 * time.Second, 100, 500 * time.Millisecond, 1 * time.Second},
		{"Extreme Load", 180 * time.Second, 200, 750 * time.Millisecond, 1500 * time.Millisecond},
		{"Spike Load", 60 * time.Second, 300, 1 * time.Second, 2 * time.Second},
		{"Gradual Increase", 300 * time.Second, 50, 400 * time.Millisecond, 800 * time.Millisecond},
		{"Burst Load", 120 * time.Second, 150, 600 * time.Millisecond, 1200 * time.Millisecond},
		{"Long Duration", 600 * time.Second, 30, 250 * time.Millisecond, 600 * time.Millisecond},
		{"Variable Load", 240 * time.Second, 75, 350 * time.Millisecond, 900 * time.Millisecond},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			limiter := rate.NewLimiter(rate.Limit(scenario.rps), 1)
			ctx, cancel := context.WithTimeout(context.Background(), scenario.duration)
			defer cancel()

			var successCount, failureCount int
			var totalLatency time.Duration
			var maxLatency time.Duration
			var minLatency = time.Hour
			var latencies []time.Duration

			for {
				select {
				case <-ctx.Done():
					avgLatency := totalLatency / time.Duration(successCount+failureCount)
					p95 := percentile(latencies, 95)
					p99 := percentile(latencies, 99)
					t.Logf("Test completed. Successes: %d, Failures: %d, Avg Latency: %v, Max Latency: %v, Min Latency: %v, P95: %v, P99: %v",
						successCount, failureCount, avgLatency, maxLatency, minLatency, p95, p99)
					assert.Greater(t, successCount, failureCount, "Success count should be greater than failure count")
					assert.Less(t, p95, scenario.maxP95, "P95 latency should be less than %v", scenario.maxP95)
					assert.Less(t, p99, scenario.maxP99, "P99 latency should be less than %v", scenario.maxP99)
					return
				default:
					err := limiter.Wait(ctx)
					if err != nil {
						continue
					}

					start := time.Now()
					resp, err := client.Get(apiURL)
					latency := time.Since(start)

					if err != nil {
						failureCount++
						continue
					}
					resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						successCount++
						totalLatency += latency
						latencies = append(latencies, latency)
						if latency > maxLatency {
							maxLatency = latency
						}
						if latency < minLatency {
							minLatency = latency
						}
					} else {
						failureCount++
					}
				}
			}
		})
	}
}

func BenchmarkAPIResponse(b *testing.B) {
	apiURL := "http://localhost:8080/api"
	client := &http.Client{Timeout: 5 * time.Second}

	endpoints := []string{"/analyze", "/generate", "/execute", "/search", "/optimize", "/refactor", "/debug"}

	for _, endpoint := range endpoints {
		b.Run(fmt.Sprintf("Endpoint: %s", endpoint), func(b *testing.B) {
			url := apiURL + endpoint
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.Get(url)
				if err != nil {
					b.Fatal(err)
				}
				_, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	}

	b.Run("AssertBenchmarkResults", func(b *testing.B) {
		for _, endpoint := range endpoints {
			result := testing.Benchmark(func(b *testing.B) {
				url := apiURL + endpoint
				for i := 0; i < b.N; i++ {
					resp, _ := client.Get(url)
					ioutil.ReadAll(resp.Body)
					resp.Body.Close()
				}
			})
			opsPerSec := float64(result.N) / result.T.Seconds()
			assert.GreaterOrEqual(b, opsPerSec, float64(150), fmt.Sprintf("Endpoint %s should handle at least 150 requests per second", endpoint))
		}
	})
}

func TestConcurrentRequests(t *testing.T) {
	apiURL := "http://localhost:8080/api"
	client := &http.Client{Timeout: 10 * time.Second}

	concurrencyLevels := []struct {
		level         int
		maxAvgLatency time.Duration
		maxP95Latency time.Duration
	}{
		{10, 100 * time.Millisecond, 200 * time.Millisecond},
		{50, 150 * time.Millisecond, 300 * time.Millisecond},
		{100, 200 * time.Millisecond, 400 * time.Millisecond},
		{200, 300 * time.Millisecond, 600 * time.Millisecond},
		{500, 500 * time.Millisecond, 1 * time.Second},
		{1000, 750 * time.Millisecond, 1500 * time.Millisecond},
		{2000, 1 * time.Second, 2 * time.Second},
	}

	for _, cl := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency: %d", cl.level), func(t *testing.T) {
			results := make(chan struct {
				err      error
				duration time.Duration
			}, cl.level)

			var wg sync.WaitGroup
			wg.Add(cl.level)

			for i := 0; i < cl.level; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					resp, err := client.Get(apiURL)
					duration := time.Since(start)
					if err == nil {
						resp.Body.Close()
					}
					results <- struct {
						err      error
						duration time.Duration
					}{err, duration}
				}()
			}

			wg.Wait()
			close(results)

			var successCount, failureCount int
			var totalDuration time.Duration
			var latencies []time.Duration

			for result := range results {
				if result.err != nil {
					failureCount++
				} else {
					successCount++
					totalDuration += result.duration
					latencies = append(latencies, result.duration)
				}
			}

			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			avgDuration := totalDuration / time.Duration(successCount)
			p50 := percentile(latencies, 50)
			p95 := percentile(latencies, 95)
			p99 := percentile(latencies, 99)

			t.Logf("Concurrent requests completed. Successes: %d, Failures: %d, Avg Duration: %v, P50: %v, P95: %v, P99: %v",
				successCount, failureCount, avgDuration, p50, p95, p99)
			assert.Greater(t, successCount, failureCount, "Success count should be greater than failure count")
			assert.Less(t, avgDuration, cl.maxAvgLatency, "Average latency should be less than %v", cl.maxAvgLatency)
			assert.Less(t, p95, cl.maxP95Latency, "P95 latency should be less than %v", cl.maxP95Latency)
		})
	}
}

func TestAPIEndpoints(t *testing.T) {
	endpoints := []struct {
		path           string
		expectedFields []string
		method         string
		payload        string
	}{
		{"/api/analyze", []string{"analysis", "confidence", "suggestions"}, "POST", `{"code": "print('Hello, World!')"}`},
		{"/api/generate", []string{"code", "language", "complexity"}, "POST", `{"prompt": "Create a function to calculate factorial"}`},
		{"/api/execute", []string{"output", "executionTime", "memoryUsage"}, "POST", `{"code": "print(sum(range(10)))"}`},
		{"/api/search", []string{"results", "totalCount", "executionTime"}, "GET", ""},
		{"/api/optimize", []string{"optimizedCode", "performanceGain"}, "POST", `{"code": "for i in range(1000000): pass"}`},
		{"/api/refactor", []string{"refactoredCode", "changes"}, "POST", `{"code": "def foo():\n    print('bar')\n    print('baz')"}`},
		{"/api/debug", []string{"issues", "suggestions", "lineNumbers"}, "POST", `{"code": "x = 10\nif x = 5:\n    print('Equal')"}`},
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("Endpoint: %s", endpoint.path), func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:8080%s", endpoint.path)
			var resp *http.Response
			var err error

			if endpoint.method == "GET" {
				resp, err = client.Get(url)
			} else {
				resp, err = client.Post(url, "application/json", nil)
			}

			assert.NoError(t, err, "Request should not fail")
			if err == nil {
				defer resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode, "Status code should be 200 OK")

				body, err := ioutil.ReadAll(resp.Body)
				assert.NoError(t, err, "Should be able to read response body")

				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Response should be valid JSON")
				assert.NotEmpty(t, result, "Response should not be empty")

				for _, field := range endpoint.expectedFields {
					assert.Contains(t, result, field, fmt.Sprintf("%s should be present in the response", field))
				}

				// Additional checks for specific endpoints
				switch endpoint.path {
				case "/api/analyze":
					assert.IsType(t, float64(0), result["confidence"], "Confidence should be a number")
				case "/api/generate":
					assert.NotEmpty(t, result["code"], "Generated code should not be empty")
				case "/api/execute":
					assert.IsType(t, float64(0), result["executionTime"], "Execution time should be a number")
				case "/api/search":
					assert.IsType(t, float64(0), result["totalCount"], "Total count should be a number")
				}
			}
		})
	}
}

func TestAPILatency(t *testing.T) {
	apiURL := "http://localhost:8080/api"
	client := &http.Client{Timeout: 10 * time.Second}

	numRequests := 2000
	maxLatency := 200 * time.Millisecond

	var latencies []time.Duration

	for i := 0; i < numRequests; i++ {
		start := time.Now()
		resp, err := client.Get(apiURL)
		latency := time.Since(start)

		assert.NoError(t, err, "Request should not fail")
		if err == nil {
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "Status code should be 200 OK")
			latencies = append(latencies, latency)
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	p50 := percentile(latencies, 50)
	p90 := percentile(latencies, 90)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)

	t.Logf("Latency percentiles: P50: %v, P90: %v, P95: %v, P99: %v", p50, p90, p95, p99)
	assert.Less(t, p50, maxLatency, "50th percentile latency should be less than %v", maxLatency)
	assert.Less(t, p90, 2*maxLatency, "90th percentile latency should be less than %v", 2*maxLatency)
	assert.Less(t, p95, 2.5*maxLatency, "95th percentile latency should be less than %v", 2.5*maxLatency)
	assert.Less(t, p99, 3*maxLatency, "99th percentile latency should be less than %v", 3*maxLatency)
}

func TestAPIStability(t *testing.T) {
	apiURL := "http://localhost:8080/api"
	client := &http.Client{Timeout: 10 * time.Second}
	duration := 10 * time.Minute
	requestInterval := 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var successCount, failureCount int
	var totalLatency time.Duration
	var latencies []time.Duration

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			avgLatency := totalLatency / time.Duration(successCount+failureCount)
			p95 := percentile(latencies, 95)
			p99 := percentile(latencies, 99)
			t.Log