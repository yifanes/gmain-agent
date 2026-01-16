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

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.handleErrorResponse(resp)
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
