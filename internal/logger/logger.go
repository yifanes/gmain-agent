package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger handles structured logging to file
type Logger struct {
	file   *os.File
	mu     sync.Mutex
	pretty bool
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp   string                 `json:"timestamp"`
	Type        string                 `json:"type"` // "request", "response", "tool_call", "tool_result", "error"
	Direction   string                 `json:"direction,omitempty"` // "outgoing", "incoming"
	Method      string                 `json:"method,omitempty"`
	URL         string                 `json:"url,omitempty"`
	StatusCode  int                    `json:"status_code,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	RequestBody map[string]interface{} `json:"request_body,omitempty"`
	ResponseBody interface{}           `json:"response_body,omitempty"`
	ToolName    string                 `json:"tool_name,omitempty"`
	ToolID      string                 `json:"tool_id,omitempty"`
	ToolInput   interface{}            `json:"tool_input,omitempty"`
	ToolResult  string                 `json:"tool_result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

var (
	globalLogger *Logger
	once         sync.Once
)

// InitLogger initializes the global logger
func InitLogger(logDir string, pretty bool) error {
	var err error
	once.Do(func() {
		globalLogger, err = NewLogger(logDir, pretty)
	})
	return err
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	return globalLogger
}

// NewLogger creates a new logger that writes to a file in the specified directory
func NewLogger(logDir string, pretty bool) (*Logger, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Generate log filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(logDir, fmt.Sprintf("claude-agent-%s.log", timestamp))

	// Open log file
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	logger := &Logger{
		file:   file,
		pretty: pretty,
	}

	// Write header
	header := fmt.Sprintf("=== Claude Agent Log Started at %s ===\n", time.Now().Format(time.RFC3339))
	file.WriteString(header)
	file.WriteString(fmt.Sprintf("Log file: %s\n\n", filename))

	return logger, nil
}

// Log writes a log entry
func (l *Logger) Log(entry LogEntry) error {
	if l == nil || l.file == nil {
		return nil // Logging disabled
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not already set
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339Nano)
	}

	// Marshal to JSON
	var data []byte
	var err error
	if l.pretty {
		data, err = json.MarshalIndent(entry, "", "  ")
	} else {
		data, err = json.Marshal(entry)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Write to file
	_, err = l.file.Write(data)
	if err != nil {
		return err
	}
	_, err = l.file.WriteString("\n")
	if err != nil {
		return err
	}

	// Flush to ensure data is written
	return l.file.Sync()
}

// LogAPIRequest logs an outgoing API request
func (l *Logger) LogAPIRequest(method, url string, headers map[string]string, body interface{}) error {
	entry := LogEntry{
		Type:      "api_request",
		Direction: "outgoing",
		Method:    method,
		URL:       url,
		Headers:   sanitizeHeaders(headers),
	}

	// Parse body if it's JSON
	if bodyMap, ok := body.(map[string]interface{}); ok {
		entry.RequestBody = bodyMap
	} else if bodyBytes, ok := body.([]byte); ok {
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			entry.RequestBody = bodyMap
		}
	}

	return l.Log(entry)
}

// LogAPIResponse logs an incoming API response
func (l *Logger) LogAPIResponse(statusCode int, headers map[string]string, body interface{}, duration time.Duration) error {
	entry := LogEntry{
		Type:       "api_response",
		Direction:  "incoming",
		StatusCode: statusCode,
		Headers:    sanitizeHeaders(headers),
		Duration:   duration.String(),
	}

	// Handle different body types
	switch v := body.(type) {
	case map[string]interface{}:
		entry.ResponseBody = v
	case []byte:
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(v, &bodyMap); err == nil {
			entry.ResponseBody = bodyMap
		} else {
			entry.ResponseBody = string(v)
		}
	case string:
		entry.ResponseBody = v
	default:
		entry.ResponseBody = body
	}

	return l.Log(entry)
}

// LogStreamChunk logs a streaming response chunk
func (l *Logger) LogStreamChunk(chunkType string, data interface{}) error {
	entry := LogEntry{
		Type:      "stream_chunk",
		Direction: "incoming",
		Metadata: map[string]interface{}{
			"chunk_type": chunkType,
			"data":       data,
		},
	}
	return l.Log(entry)
}

// LogToolCall logs a tool call
func (l *Logger) LogToolCall(toolName, toolID string, input interface{}) error {
	entry := LogEntry{
		Type:      "tool_call",
		Direction: "outgoing",
		ToolName:  toolName,
		ToolID:    toolID,
		ToolInput: input,
	}
	return l.Log(entry)
}

// LogToolResult logs a tool execution result
func (l *Logger) LogToolResult(toolName, toolID string, result string, isError bool, duration time.Duration) error {
	entry := LogEntry{
		Type:       "tool_result",
		Direction:  "incoming",
		ToolName:   toolName,
		ToolID:     toolID,
		ToolResult: truncateString(result, 10000), // Limit result size
		Duration:   duration.String(),
	}
	if isError {
		entry.Error = "tool execution failed"
	}
	return l.Log(entry)
}

// LogError logs an error
func (l *Logger) LogError(errorType string, err error, metadata map[string]interface{}) error {
	entry := LogEntry{
		Type:     "error",
		Error:    err.Error(),
		Metadata: metadata,
	}
	return l.Log(entry)
}

// Close closes the log file
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Write footer
	footer := fmt.Sprintf("\n=== Claude Agent Log Ended at %s ===\n", time.Now().Format(time.RFC3339))
	l.file.WriteString(footer)

	return l.file.Close()
}

// sanitizeHeaders removes sensitive information from headers
func sanitizeHeaders(headers map[string]string) map[string]string {
	sanitized := make(map[string]string)
	for k, v := range headers {
		switch k {
		case "x-api-key", "Authorization":
			// Mask sensitive values
			if len(v) > 10 {
				sanitized[k] = v[:10] + "..." + v[len(v)-4:]
			} else {
				sanitized[k] = "***"
			}
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (truncated, total length: %d)", len(s))
}
