package tools

import (
	"agent-go/pkg/core"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// SearchTool provides web search capabilities
type SearchTool struct {
	apiKey      string
	cx          string
	client      *http.Client
	rateLimiter *RateLimiter
}

// RateLimiter implements a simple rate limiting mechanism
type RateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	interval time.Duration
}

// NewRateLimiter creates a new rate limiter with specified interval
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		interval: interval,
	}
}

// Wait blocks until rate limit allows next call
func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.lastCall.IsZero() {
		elapsed := time.Since(r.lastCall)
		if elapsed < r.interval {
			time.Sleep(r.interval - elapsed)
		}
	}
	r.lastCall = time.Now()
}

// NewSearchTool creates a new search tool instance
func NewSearchTool(apiKey, cx string) *SearchTool {
	return &SearchTool{
		apiKey: apiKey,
		cx:     cx,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
		rateLimiter: NewRateLimiter(time.Second), // 1 request per second
	}
}

func (t *SearchTool) Name() string {
	return "search"
}

func (t *SearchTool) Description() string {
	return "Search the web using Google Custom Search API"
}

type SearchRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results,omitempty"`
	Language   string `json:"language,omitempty"`
	SafeSearch bool   `json:"safe_search,omitempty"`
	SiteFilter string `json:"site_filter,omitempty"`
}

type SearchResult struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	DisplayLink string `json:"display_link"`
}

func (t *SearchTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"query": {
			Type:        "string",
			Description: "Search query",
			Required:    true,
		},
		"num_results": {
			Type:        "integer",
			Description: "Number of results to return (max 10)",
			Required:    false,
		},
		"language": {
			Type:        "string",
			Description: "Language code for results (e.g., 'en')",
			Required:    false,
		},
		"safe_search": {
			Type:        "boolean",
			Description: "Whether to enable safe search",
			Required:    false,
		},
		"site_filter": {
			Type:        "string",
			Description: "Limit results to specific site (e.g., 'site:example.com')",
			Required:    false,
		},
	}
}

func (t *SearchTool) Execute(input string) (string, error) {
	var req SearchRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("failed to parse search request: %w", err)
	}

	if req.Query == "" {
		return "", fmt.Errorf("search query is required")
	}

	if req.NumResults <= 0 || req.NumResults > 10 {
		req.NumResults = 5 // Default to 5 results
	}

	// Build search query
	query := req.Query
	if req.SiteFilter != "" {
		query = fmt.Sprintf("%s site:%s", query, req.SiteFilter)
	}

	// Wait for rate limiter
	t.rateLimiter.Wait()

	// Build request URL
	params := url.Values{}
	params.Add("key", t.apiKey)
	params.Add("cx", t.cx)
	params.Add("q", query)
	params.Add("num", fmt.Sprintf("%d", req.NumResults))

	if req.Language != "" {
		params.Add("lr", fmt.Sprintf("lang_%s", req.Language))
	}
	if req.SafeSearch {
		params.Add("safe", "active")
	}

	searchURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?%s", params.Encode())

	// Execute request
	resp, err := t.client.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Items []struct {
			Title       string `json:"title"`
			Link        string `json:"link"`
			Snippet     string `json:"snippet"`
			DisplayLink string `json:"displayLink"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	// Format results
	searchResults := make([]SearchResult, 0, len(result.Items))
	for _, item := range result.Items {
		searchResults = append(searchResults, SearchResult{
			Title:       item.Title,
			Link:        item.Link,
			Snippet:     item.Snippet,
			DisplayLink: item.DisplayLink,
		})
	}

	formatted, err := json.MarshalIndent(searchResults, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format results: %w", err)
	}

	return string(formatted), nil
}
