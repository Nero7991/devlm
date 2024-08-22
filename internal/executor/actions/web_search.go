package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
	"sync"
)

type SearchResult struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"snippet"`
}

type WebSearcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

type GoogleSearcher struct {
	apiKey      string
	cx          string
	httpClient  *http.Client
	rateLimiter *time.Ticker
	mu          sync.Mutex
	cache       map[string]cachedResult
	cacheMu     sync.RWMutex
}

type cachedResult struct {
	results []SearchResult
	expiry  time.Time
}

func NewGoogleSearcher(apiKey, cx string) (*GoogleSearcher, error) {
	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("API key and custom search engine ID must not be empty")
	}
	return &GoogleSearcher{
		apiKey: apiKey,
		cx:     cx,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: time.NewTicker(time.Second / 10), // 10 requests per second
		cache:       make(map[string]cachedResult),
	}, nil
}

func (gs *GoogleSearcher) Search(ctx context.Context, query string) ([]SearchResult, error) {
	// Check cache first
	gs.cacheMu.RLock()
	if cachedRes, ok := gs.cache[query]; ok && time.Now().Before(cachedRes.expiry) {
		gs.cacheMu.RUnlock()
		return cachedRes.results, nil
	}
	gs.cacheMu.RUnlock()

	gs.mu.Lock()
	<-gs.rateLimiter.C
	gs.mu.Unlock()

	baseURL := "https://www.googleapis.com/customsearch/v1"
	params := url.Values{}
	params.Set("key", gs.apiKey)
	params.Set("cx", gs.cx)
	params.Set("q", query)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := gs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Items []SearchResult `json:"items"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Cache the results with expiration
	gs.cacheMu.Lock()
	gs.cache[query] = cachedResult{
		results: result.Items,
		expiry:  time.Now().Add(24 * time.Hour), // Cache expires after 24 hours
	}
	gs.cacheMu.Unlock()

	return result.Items, nil
}

type WebSearch struct {
	searcher WebSearcher
}

func NewWebSearch(searcher WebSearcher) (*WebSearch, error) {
	if searcher == nil {
		return nil, fmt.Errorf("searcher must not be nil")
	}
	return &WebSearch{
		searcher: searcher,
	}, nil
}

func (ws *WebSearch) Search(ctx context.Context, query string, numResults int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	if numResults <= 0 {
		numResults = 5
	}

	results, err := ws.searcher.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return []SearchResult{}, nil
	}

	return results[:min(numResults, len(results))], nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}