package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	WebFetchTimeout    = 30 * time.Second
	MaxWebFetchSize    = 1024 * 1024 // 1MB
	MaxWebFetchContent = 50000       // Characters
)

// WebFetchTool fetches content from URLs
type WebFetchTool struct {
	httpClient *http.Client
}

// NewWebFetchTool creates a new WebFetch tool
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		httpClient: &http.Client{
			Timeout: WebFetchTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (t *WebFetchTool) Name() string {
	return "WebFetch"
}

func (t *WebFetchTool) Description() string {
	return `Fetches content from a specified URL and returns the text content.

Usage notes:
- The URL must be a fully-formed valid URL
- HTTP URLs will be automatically upgraded to HTTPS
- Results may be summarized if the content is very large
- This tool is read-only and does not modify any files`
}

func (t *WebFetchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"format":      "uri",
				"description": "The URL to fetch content from",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The prompt to run on the fetched content (currently returns raw content)",
			},
		},
		"required": []string{"url", "prompt"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	urlStr, ok := GetString(params, "url")
	if !ok || urlStr == "" {
		return NewErrorResultString("url parameter is required"), nil
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return NewErrorResultString(fmt.Sprintf("Invalid URL: %s", err.Error())), nil
	}

	// Upgrade HTTP to HTTPS
	if parsedURL.Scheme == "http" {
		parsedURL.Scheme = "https"
	}

	if parsedURL.Scheme != "https" {
		return NewErrorResultString("Only HTTP/HTTPS URLs are supported"), nil
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return NewErrorResult(err), nil
	}

	req.Header.Set("User-Agent", "Claude-Code-Go/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")

	// Fetch URL
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return NewErrorResultString(fmt.Sprintf("Failed to fetch URL: %s", err.Error())), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NewErrorResultString(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)), nil
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, MaxWebFetchSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return NewErrorResultString(fmt.Sprintf("Failed to read response: %s", err.Error())), nil
	}

	// Convert to text (basic HTML to text conversion)
	content := string(body)
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "text/html") {
		content = htmlToText(content)
	}

	// Truncate if necessary
	if len(content) > MaxWebFetchContent {
		content = content[:MaxWebFetchContent] + "\n\n... (content truncated)"
	}

	return NewResult(content), nil
}

// htmlToText performs basic HTML to text conversion
func htmlToText(html string) string {
	// Remove script and style elements
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRe := regexp.MustCompile(`(?is)<!--.*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// Convert common block elements to newlines
	blockRe := regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|br|hr)[^>]*>`)
	html = blockRe.ReplaceAllString(html, "\n")

	// Convert br tags
	brRe := regexp.MustCompile(`(?i)<br[^>]*>`)
	html = brRe.ReplaceAllString(html, "\n")

	// Remove all remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	html = tagRe.ReplaceAllString(html, "")

	// Decode common HTML entities
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = strings.ReplaceAll(html, "&apos;", "'")

	// Normalize whitespace
	whitespaceRe := regexp.MustCompile(`[ \t]+`)
	html = whitespaceRe.ReplaceAllString(html, " ")

	// Normalize newlines
	newlineRe := regexp.MustCompile(`\n[ \t]+`)
	html = newlineRe.ReplaceAllString(html, "\n")

	multiNewlineRe := regexp.MustCompile(`\n{3,}`)
	html = multiNewlineRe.ReplaceAllString(html, "\n\n")

	return strings.TrimSpace(html)
}
