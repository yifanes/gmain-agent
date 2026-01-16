package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/claude-code-go/internal/logger"
	"github.com/anthropics/claude-code-go/internal/retry"
)

const (
	// DefaultBaseURL is the base URL without /v1 path (Claude Code compatible)
	DefaultBaseURL    = "https://api.anthropic.com"
	DefaultModel      = "claude-sonnet-4-20250514"
	DefaultMaxTokens  = 8192
	AnthropicVersion  = "2023-06-01"
	DefaultTimeout    = 5 * time.Minute
	// MessagesEndpoint is the API endpoint for messages
	MessagesEndpoint  = "v1/messages"
)

// AuthType represents the type of authentication
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeBearer AuthType = "bearer"
)

// Client is the Anthropic API client
type Client struct {
	credential string
	authType   AuthType
	baseURL    string
	httpClient *http.Client
	retrier    *retry.Retrier
	model      string
	maxTokens  int
}

// ClientOption is a function that configures the client
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the API
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithModel sets the model to use
func WithModel(model string) ClientOption {
	return func(c *Client) {
		c.model = model
	}
}

// WithMaxTokens sets the max tokens for responses
func WithMaxTokens(maxTokens int) ClientOption {
	return func(c *Client) {
		c.maxTokens = maxTokens
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithAuthType sets the authentication type
func WithAuthType(authType AuthType) ClientOption {
	return func(c *Client) {
		c.authType = authType
	}
}

// NewClient creates a new Anthropic API client
// credential can be either an API key or a Bearer token depending on authType
func NewClient(credential string, opts ...ClientOption) *Client {
	c := &Client{
		credential: credential,
		authType:   AuthTypeAPIKey, // Default to API key authentication
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		retrier: retry.NewRetrier(),
		model:     DefaultModel,
		maxTokens: DefaultMaxTokens,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GetModel returns the current model
func (c *Client) GetModel() string {
	return c.model
}

// GetBaseURL returns the current base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// buildURL constructs the full API endpoint URL
func (c *Client) buildURL(endpoint string) string {
	base := strings.TrimSuffix(c.baseURL, "/")
	endpoint = strings.TrimPrefix(endpoint, "/")
	return base + "/" + endpoint
}

// CreateMessage sends a non-streaming message request
func (c *Client) CreateMessage(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.buildURL(MessagesEndpoint), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	// Log request
	if log := logger.GetLogger(); log != nil {
		headers := make(map[string]string)
		for k, v := range httpReq.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
		var bodyMap map[string]interface{}
		json.Unmarshal(body, &bodyMap)
		log.LogAPIRequest("POST", httpReq.URL.String(), headers, bodyMap)
	}

	startTime := time.Now()

	// Use retrier to handle retries
	resp, err := c.retrier.Do(ctx, func() (*http.Response, error) {
		return c.httpClient.Do(httpReq)
	})

	duration := time.Since(startTime)

	if err != nil {
		if log := logger.GetLogger(); log != nil {
			log.LogError("http_request_failed", err, map[string]interface{}{
				"url":      httpReq.URL.String(),
				"duration": duration.String(),
			})
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log response
	if log := logger.GetLogger(); log != nil {
		respHeaders := make(map[string]string)
		for k, v := range resp.Header {
			if len(v) > 0 {
				respHeaders[k] = v[0]
			}
		}
		var respBodyMap map[string]interface{}
		json.Unmarshal(respBody, &respBodyMap)
		log.LogAPIResponse(resp.StatusCode, respHeaders, respBodyMap, duration)
	}

	var result MessagesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StreamMessage sends a streaming message request
func (c *Client) StreamMessage(ctx context.Context, req *MessagesRequest) (*StreamReader, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.buildURL(MessagesEndpoint), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	// Log request
	if log := logger.GetLogger(); log != nil {
		headers := make(map[string]string)
		for k, v := range httpReq.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
		var bodyMap map[string]interface{}
		json.Unmarshal(body, &bodyMap)
		log.LogAPIRequest("POST", httpReq.URL.String(), headers, bodyMap)
	}

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if log := logger.GetLogger(); log != nil {
			log.LogError("http_request_failed", err, map[string]interface{}{
				"url":      httpReq.URL.String(),
				"duration": time.Since(startTime).String(),
			})
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		if log := logger.GetLogger(); log != nil {
			respHeaders := make(map[string]string)
			for k, v := range resp.Header {
				if len(v) > 0 {
					respHeaders[k] = v[0]
				}
			}
			log.LogAPIResponse(resp.StatusCode, respHeaders, "error response", time.Since(startTime))
		}
		return nil, c.handleErrorResponse(resp)
	}

	// Log that streaming started
	if log := logger.GetLogger(); log != nil {
		respHeaders := make(map[string]string)
		for k, v := range resp.Header {
			if len(v) > 0 {
				respHeaders[k] = v[0]
			}
		}
		log.LogAPIResponse(resp.StatusCode, respHeaders, "stream_started", time.Since(startTime))
	}

	return NewStreamReader(resp.Body), nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	// Set authentication header based on auth type
	switch c.authType {
	case AuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+c.credential)
	default:
		// Default to x-api-key for standard Anthropic API
		req.Header.Set("x-api-key", c.credential)
	}

	req.Header.Set("anthropic-version", AnthropicVersion)
}

func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("API error (%d): %s - %s", resp.StatusCode, errResp.Error.Type, errResp.Error.Message)
	}

	return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
}
