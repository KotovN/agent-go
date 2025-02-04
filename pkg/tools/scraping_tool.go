package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KotovN/agent-go/pkg/core"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt"
)

// ScrapingTool provides web scraping capabilities
type ScrapingTool struct {
	client      *http.Client
	rateLimiter *RateLimiter
	userAgent   string
	robotsCache map[string]*robotstxt.RobotsData
}

// NewScrapingTool creates a new web scraping tool instance
func NewScrapingTool(userAgent string) *ScrapingTool {
	if userAgent == "" {
		userAgent = "agent-go/1.0"
	}
	return &ScrapingTool{
		client: &http.Client{
			Timeout: time.Second * 30,
		},
		rateLimiter: NewRateLimiter(time.Second), // 1 request per second
		userAgent:   userAgent,
		robotsCache: make(map[string]*robotstxt.RobotsData),
	}
}

func (t *ScrapingTool) Name() string {
	return "scrape"
}

func (t *ScrapingTool) Description() string {
	return "Extract data from web pages with respect to robots.txt rules"
}

type ScrapingRequest struct {
	URL       string            `json:"url"`
	Selectors map[string]string `json:"selectors"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
}

type ScrapingResult struct {
	URL   string            `json:"url"`
	Data  map[string]string `json:"data"`
	Title string            `json:"title,omitempty"`
	Error string            `json:"error,omitempty"`
}

func (t *ScrapingTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"url": {
			Type:        "string",
			Description: "URL of the web page to scrape",
			Required:    true,
		},
		"selectors": {
			Type:        "object",
			Description: "Map of CSS selectors to extract data (key: name, value: selector)",
			Required:    true,
		},
		"headers": {
			Type:        "object",
			Description: "Custom HTTP headers for the request",
			Required:    false,
		},
		"timeout": {
			Type:        "integer",
			Description: "Request timeout in seconds",
			Required:    false,
		},
	}
}

func (t *ScrapingTool) Execute(input string) (string, error) {
	var req ScrapingRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("failed to parse scraping request: %w", err)
	}

	if req.URL == "" {
		return "", fmt.Errorf("URL is required")
	}

	if len(req.Selectors) == 0 {
		return "", fmt.Errorf("at least one selector is required")
	}

	// Parse URL
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Check robots.txt
	if allowed, err := t.checkRobotsTxt(parsedURL); err != nil {
		return "", fmt.Errorf("failed to check robots.txt: %w", err)
	} else if !allowed {
		return "", fmt.Errorf("URL is not allowed by robots.txt")
	}

	// Wait for rate limiter
	t.rateLimiter.Wait()

	// Create request
	httpReq, err := http.NewRequest("GET", req.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("User-Agent", t.userAgent)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set timeout if specified
	if req.Timeout > 0 {
		t.client.Timeout = time.Duration(req.Timeout) * time.Second
	}

	// Execute request
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract data using selectors
	result := ScrapingResult{
		URL:  req.URL,
		Data: make(map[string]string),
	}

	// Get page title
	result.Title = doc.Find("title").Text()

	// Extract data for each selector
	for name, selector := range req.Selectors {
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			// Get text content and clean it up
			text := strings.TrimSpace(selection.First().Text())
			result.Data[name] = text
		}
	}

	// Format result
	formatted, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format result: %w", err)
	}

	return string(formatted), nil
}

func (t *ScrapingTool) checkRobotsTxt(parsedURL *url.URL) (bool, error) {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsedURL.Scheme, parsedURL.Host)

	// Check cache first
	if robots, ok := t.robotsCache[parsedURL.Host]; ok {
		return robots.TestAgent(parsedURL.Path, t.userAgent), nil
	}

	// Fetch robots.txt
	resp, err := http.Get(robotsURL)
	if err != nil {
		return true, nil // Assume allowed if robots.txt is unavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return true, nil // Assume allowed if robots.txt is unavailable
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, nil
	}

	robots, err := robotstxt.FromString(string(body))
	if err != nil {
		return true, nil
	}

	// Cache the robots.txt data
	t.robotsCache[parsedURL.Host] = robots

	return robots.TestAgent(parsedURL.Path, t.userAgent), nil
}
