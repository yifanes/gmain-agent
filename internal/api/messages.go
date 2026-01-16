package api

import "encoding/json"

// Role represents the role of a message sender
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentType represents the type of content in a message
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeImage      ContentType = "image"
)

// Content represents a single content block in a message
type Content struct {
	Type      ContentType     `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role    Role      `json:"role"`
	Content []Content `json:"content"`
}

// NewTextMessage creates a new text message
func NewTextMessage(role Role, text string) Message {
	return Message{
		Role: role,
		Content: []Content{
			{Type: ContentTypeText, Text: text},
		},
	}
}

// NewToolResultMessage creates a new tool result message
func NewToolResultMessage(toolUseID string, result string, isError bool) Message {
	return Message{
		Role: RoleUser,
		Content: []Content{
			{
				Type:      ContentTypeToolResult,
				ToolUseID: toolUseID,
				Content:   result,
				IsError:   isError,
			},
		},
	}
}

// Tool represents a tool definition for Claude
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// MessagesRequest represents a request to the Messages API
type MessagesRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// MessagesResponse represents a non-streaming response from the Messages API
type MessagesResponse struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Role         Role      `json:"role"`
	Content      []Content `json:"content"`
	Model        string    `json:"model"`
	StopReason   string    `json:"stop_reason"`
	StopSequence string    `json:"stop_sequence,omitempty"`
	Usage        Usage     `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent represents an event in the streaming response
type StreamEvent struct {
	Type         string          `json:"type"`
	Index        int             `json:"index,omitempty"`
	Delta        *Delta          `json:"delta,omitempty"`
	ContentBlock *Content        `json:"content_block,omitempty"`
	Message      json.RawMessage `json:"message,omitempty"`
	Usage        *Usage          `json:"usage,omitempty"`
}

// Delta represents incremental content in a streaming response
type Delta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
