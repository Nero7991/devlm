package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type LLMClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	cache      map[string]*LLMResponse
}

type LLMRequest struct {
	Prompt      string   `json:"prompt"`
	MaxTokens   int      `json:"max_tokens"`
	Temperature float64  `json:"temperature"`
	StopWords   []string `json:"stop_words,omitempty"`
	Language    string   `json:"language,omitempty"`
}

type LLMResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

type ClientOption func(*LLMClient)

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *LLMClient) {
		c.httpClient.Timeout = timeout
	}
}

func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *LLMClient) {
		c.httpClient.Transport = &retryRoundTripper{
			maxRetries: maxRetries,
			transport:  http.DefaultTransport,
		}
	}
}

func WithCacheSize(size int) ClientOption {
	return func(c *LLMClient) {
		c.cache = make(map[string]*LLMResponse, size)
	}
}

func NewLLMClient(apiKey, baseURL string, options ...ClientOption) *LLMClient {
	client := &LLMClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		cache: make(map[string]*LLMResponse),
	}

	for _, option := range options {
		option(client)
	}

	return client
}

func (c *LLMClient) GenerateCode(ctx context.Context, prompt string, maxTokens int, temperature float64, language string) (string, error) {
	req := LLMRequest{
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Language:    language,
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate code: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no code generated")
	}

	return strings.TrimSpace(resp.Choices[0].Text), nil
}

func (c *LLMClient) AnalyzeRequirements(ctx context.Context, requirements string, outputFormat string) (string, error) {
	prompt := fmt.Sprintf("Analyze the following project requirements:\n\n%s\n\nProvide a summary of the key features and tasks in the following format: %s", requirements, outputFormat)
	req := LLMRequest{
		Prompt:      prompt,
		MaxTokens:   500,
		Temperature: 0.5,
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to analyze requirements: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no analysis generated")
	}

	return strings.TrimSpace(resp.Choices[0].Text), nil
}

func (c *LLMClient) ExplainCode(ctx context.Context, code string, detailLevel string, language string) (string, error) {
	prompt := fmt.Sprintf("Explain the following %s code with a %s level of detail:\n\n%s\n\nProvide a detailed explanation:", language, detailLevel, code)
	req := LLMRequest{
		Prompt:      prompt,
		MaxTokens:   1000,
		Temperature: 0.3,
		Language:    language,
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to explain code: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no explanation generated")
	}

	return strings.TrimSpace(resp.Choices[0].Text), nil
}

type Improvement struct {
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	Priority    string `json:"priority"`
}

func (c *LLMClient) SuggestImprovements(ctx context.Context, code string, categories []string, priorities []string) ([]Improvement, error) {
	prompt := fmt.Sprintf("Suggest improvements for the following code:\n\n%s\n\nProvide detailed suggestions in JSON format with 'description', 'suggestion', and 'priority' fields. Focus on these categories: %s. Use these priority levels: %s", code, strings.Join(categories, ", "), strings.Join(priorities, ", "))
	req := LLMRequest{
		Prompt:      prompt,
		MaxTokens:   800,
		Temperature: 0.6,
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest improvements: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no suggestions generated")
	}

	var improvements []Improvement
	err = json.Unmarshal([]byte(resp.Choices[0].Text), &improvements)
	if err != nil {
		return nil, fmt.Errorf("failed to parse suggestions: %w", err)
	}

	return improvements, nil
}

func (c *LLMClient) sendRequest(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	cacheKey := fmt.Sprintf("%s-%d-%.2f-%s", req.Prompt, req.MaxTokens, req.Temperature, req.Language)
	if cachedResp, ok := c.cache[cacheKey]; ok {
		return cachedResp, nil
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var llmResp LLMResponse
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if llmResp.Error.Message != "" {
		return nil, fmt.Errorf("LLM API error (%s): %s", llmResp.Error.Code, llmResp.Error.Message)
	}

	c.cache[cacheKey] = &llmResp
	return &llmResp, nil
}

type retryRoundTripper struct {
	maxRetries int
	transport  http.RoundTripper
}

func (rrt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= rrt.maxRetries; attempt++ {
		resp, err = rrt.transport.RoundTrip(req)
		if err == nil && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt == rrt.maxRetries {
			break
		}

		backoff := time.Duration(1<<uint(attempt)) * time.Second
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", rrt.maxRetries, err)
}