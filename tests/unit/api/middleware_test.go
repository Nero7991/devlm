package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nero7991/devlm/internal/config"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	return args.Get(0).(*redis.BoolCmd)
}

func TestCORSMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	middleware := CORSMiddleware(handler)

	testCases := []struct {
		name            string
		method          string
		origin          string
		expectedStatus  int
		expectedHeaders map[string]string
	}{
		{
			name:           "GET request",
			method:         "GET",
			origin:         "https://example.com",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "POST, GET, OPTIONS, PUT, DELETE",
				"Access-Control-Allow-Headers": "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
			},
		},
		{
			name:           "OPTIONS request",
			method:         "OPTIONS",
			origin:         "https://test.com",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://test.com",
				"Access-Control-Allow-Methods": "POST, GET, OPTIONS, PUT, DELETE",
				"Access-Control-Allow-Headers": "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
			},
		},
		{
			name:           "POST request with no origin",
			method:         "POST",
			origin:         "",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, GET, OPTIONS, PUT, DELETE",
				"Access-Control-Allow-Headers": "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, "/", nil)
			assert.NoError(t, err)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			for key, value := range tc.expectedHeaders {
				assert.Equal(t, value, rr.Header().Get(key))
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware(handler)

	testCases := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{"Valid token", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", http.StatusOK},
		{"Invalid token", "Bearer invalid_token", http.StatusUnauthorized},
		{"Missing token", "", http.StatusUnauthorized},
		{"Malformed token", "InvalidTokenFormat", http.StatusUnauthorized},
		{"Empty Bearer token", "Bearer ", http.StatusUnauthorized},
		{"Token with spaces", "Bearer token with spaces", http.StatusUnauthorized},
		{"Token with special characters", "Bearer token$with@special#chars", http.StatusUnauthorized},
		{"Very long token", "Bearer " + string(make([]byte, 1000)), http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/", nil)
			assert.NoError(t, err)

			if tc.token != "" {
				req.Header.Set("Authorization", tc.token)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(nil) // Discard logs for testing

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := LoggingMiddleware(logger)(handler)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"GET request", "GET", "/test", http.StatusOK},
		{"POST request", "POST", "/api/data", http.StatusOK},
		{"PUT request", "PUT", "/update", http.StatusOK},
		{"DELETE request", "DELETE", "/delete", http.StatusOK},
		{"PATCH request", "PATCH", "/patch", http.StatusOK},
		{"OPTIONS request", "OPTIONS", "/options", http.StatusOK},
		{"HEAD request", "HEAD", "/head", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.path, nil)
			assert.NoError(t, err)

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	mockRedisClient := new(MockRedisClient)

	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:   true,
			Requests:  10,
			PerSecond: 1,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := RateLimitMiddleware(mockRedisClient, cfg)(handler)

	testCases := []struct {
		name            string
		setupMock       func()
		expectedStatus  int
		expectedHeaders map[string]string
	}{
		{
			name: "Within rate limit",
			setupMock: func() {
				mockRedisClient.On("Incr", mock.Anything, mock.Anything).Return(redis.NewIntCmd(context.Background())).Run(func(args mock.Arguments) {
					cmd := args.Get(0).(*redis.IntCmd)
					cmd.SetVal(5)
				})
				mockRedisClient.On("Expire", mock.Anything, mock.Anything, mock.Anything).Return(redis.NewBoolCmd(context.Background()))
			},
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"X-RateLimit-Limit":     "10",
				"X-RateLimit-Remaining": "5",
				"X-RateLimit-Reset":     "1",
			},
		},
		{
			name: "Exceeding rate limit",
			setupMock: func() {
				mockRedisClient.On("Incr", mock.Anything, mock.Anything).Return(redis.NewIntCmd(context.Background())).Run(func(args mock.Arguments) {
					cmd := args.Get(0).(*redis.IntCmd)
					cmd.SetVal(11)
				})
				mockRedisClient.On("Expire", mock.Anything, mock.Anything, mock.Anything).Return(redis.NewBoolCmd(context.Background()))
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedHeaders: map[string]string{
				"X-RateLimit-Limit":     "10",
				"X-RateLimit-Remaining": "0",
				"X-RateLimit-Reset":     "1",
			},
		},
		{
			name: "At rate limit boundary",
			setupMock: func() {
				mockRedisClient.On("Incr", mock.Anything, mock.Anything).Return(redis.NewIntCmd(context.Background())).Run(func(args mock.Arguments) {
					cmd := args.Get(0).(*redis.IntCmd)
					cmd.SetVal(10)
				})
				mockRedisClient.On("Expire", mock.Anything, mock.Anything, mock.Anything).Return(redis.NewBoolCmd(context.Background()))
			},
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"X-RateLimit-Limit":     "10",
				"X-RateLimit-Remaining": "0",
				"X-RateLimit-Reset":     "1",
			},
		},
		{
			name: "Redis error",
			setupMock: func() {
				mockRedisClient.On("Incr", mock.Anything, mock.Anything).Return(redis.NewIntCmd(context.Background())).Run(func(args mock.Arguments) {
					cmd := args.Get(0).(*redis.IntCmd)
					cmd.SetErr(redis.ErrClosed)
				})
			},
			expectedStatus: http.StatusInternalServerError,
			expectedHeaders: map[string]string{
				"X-RateLimit-Limit":     "10",
				"X-RateLimit-Remaining": "0",
				"X-RateLimit-Reset":     "1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRedisClient.ExpectedCalls = nil
			mockRedisClient.Calls = nil
			tc.setupMock()

			req, err := http.NewRequest("GET", "/", nil)
			assert.NoError(t, err)
			req.RemoteAddr = "127.0.0.1:1234"

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			for key, value := range tc.expectedHeaders {
				assert.Equal(t, value, rr.Header().Get(key))
			}
			mockRedisClient.AssertExpectations(t)
		})
	}
}