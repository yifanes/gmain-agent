package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/anthropics/claude-code-go/internal/api"
)

// Tool defines the interface that all tools must implement
type Tool interface {
	// Name returns the unique name of the tool
	Name() string

	// Description returns a human-readable description of the tool
	Description() string

	// Parameters returns the JSON schema for the tool's parameters
	Parameters() map[string]interface{}

	// Execute runs the tool with the given parameters
	Execute(ctx context.Context, params map[string]interface{}) (*Result, error)
}

// Result represents the result of a tool execution
type Result struct {
	Output  string
	IsError bool
}

// NewResult creates a successful result
func NewResult(output string) *Result {
	return &Result{Output: output, IsError: false}
}

// NewErrorResult creates an error result
func NewErrorResult(err error) *Result {
	return &Result{Output: err.Error(), IsError: true}
}

// NewErrorResultString creates an error result from a string
func NewErrorResultString(msg string) *Result {
	return &Result{Output: msg, IsError: true}
}

// Registry manages all available tools
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToAPITools converts registered tools to API tool definitions
func (r *Registry) ToAPITools() []api.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]api.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, api.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.Parameters(),
		})
	}
	return tools
}

// Execute runs a tool by name with the given parameters
func (r *Registry) Execute(ctx context.Context, name string, params json.RawMessage) (*Result, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	var paramsMap map[string]interface{}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &paramsMap); err != nil {
			return nil, fmt.Errorf("failed to parse tool parameters: %w", err)
		}
	} else {
		paramsMap = make(map[string]interface{})
	}

	return tool.Execute(ctx, paramsMap)
}

// Helper functions for parameter extraction

// GetString extracts a string parameter
func GetString(params map[string]interface{}, key string) (string, bool) {
	v, ok := params[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetStringDefault extracts a string parameter with a default value
func GetStringDefault(params map[string]interface{}, key, defaultValue string) string {
	if s, ok := GetString(params, key); ok {
		return s
	}
	return defaultValue
}

// GetInt extracts an integer parameter
func GetInt(params map[string]interface{}, key string) (int, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

// GetIntDefault extracts an integer parameter with a default value
func GetIntDefault(params map[string]interface{}, key string, defaultValue int) int {
	if i, ok := GetInt(params, key); ok {
		return i
	}
	return defaultValue
}

// GetBool extracts a boolean parameter
func GetBool(params map[string]interface{}, key string) (bool, bool) {
	v, ok := params[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// GetBoolDefault extracts a boolean parameter with a default value
func GetBoolDefault(params map[string]interface{}, key string, defaultValue bool) bool {
	if b, ok := GetBool(params, key); ok {
		return b
	}
	return defaultValue
}

// GetStringArray extracts a string array parameter
func GetStringArray(params map[string]interface{}, key string) ([]string, bool) {
	v, ok := params[key]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result, true
}
