package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TodoStatus represents the status of a todo item
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusCompleted  TodoStatus = "completed"
)

// TodoItem represents a single todo item
type TodoItem struct {
	Content    string     `json:"content"`
	Status     TodoStatus `json:"status"`
	ActiveForm string     `json:"activeForm"`
}

// TodoList manages the current todo items
type TodoList struct {
	items []TodoItem
	mu    sync.RWMutex
}

// NewTodoList creates a new todo list
func NewTodoList() *TodoList {
	return &TodoList{
		items: make([]TodoItem, 0),
	}
}

// GetItems returns a copy of all todo items
func (t *TodoList) GetItems() []TodoItem {
	t.mu.RLock()
	defer t.mu.RUnlock()
	items := make([]TodoItem, len(t.items))
	copy(items, t.items)
	return items
}

// SetItems replaces all todo items
func (t *TodoList) SetItems(items []TodoItem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.items = items
}

// GetCurrentTask returns the current in-progress task, if any
func (t *TodoList) GetCurrentTask() *TodoItem {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, item := range t.items {
		if item.Status == TodoStatusInProgress {
			return &item
		}
	}
	return nil
}

// TodoWriteTool manages the todo list
type TodoWriteTool struct {
	todoList *TodoList
}

// NewTodoWriteTool creates a new TodoWrite tool
func NewTodoWriteTool(todoList *TodoList) *TodoWriteTool {
	return &TodoWriteTool{todoList: todoList}
}

func (t *TodoWriteTool) Name() string {
	return "TodoWrite"
}

func (t *TodoWriteTool) Description() string {
	return `Use this tool to create and manage a structured task list for your current coding session.

This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.

Task States:
- pending: Task not yet started
- in_progress: Currently working on (limit to ONE task at a time)
- completed: Task finished successfully`
}

func (t *TodoWriteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"todos": map[string]interface{}{
				"type":        "array",
				"description": "The updated todo list",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]interface{}{
							"type":      "string",
							"minLength": 1,
						},
						"status": map[string]interface{}{
							"type": "string",
							"enum": []string{"pending", "in_progress", "completed"},
						},
						"activeForm": map[string]interface{}{
							"type":      "string",
							"minLength": 1,
						},
					},
					"required": []string{"content", "status", "activeForm"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	todosRaw, ok := params["todos"]
	if !ok {
		return NewErrorResultString("todos parameter is required"), nil
	}

	// Convert to JSON and back to parse the todos
	todosJSON, err := json.Marshal(todosRaw)
	if err != nil {
		return NewErrorResult(err), nil
	}

	var todos []TodoItem
	if err := json.Unmarshal(todosJSON, &todos); err != nil {
		return NewErrorResultString(fmt.Sprintf("Invalid todos format: %s", err.Error())), nil
	}

	// Validate todos
	inProgressCount := 0
	for _, todo := range todos {
		if todo.Content == "" {
			return NewErrorResultString("Todo content cannot be empty"), nil
		}
		if todo.ActiveForm == "" {
			return NewErrorResultString("Todo activeForm cannot be empty"), nil
		}
		if todo.Status == TodoStatusInProgress {
			inProgressCount++
		}
	}

	if inProgressCount > 1 {
		return NewErrorResultString("Only one todo can be in_progress at a time"), nil
	}

	// Update the todo list
	t.todoList.SetItems(todos)

	// Build response
	var output strings.Builder
	output.WriteString("Todos have been modified successfully.")

	if len(todos) > 0 {
		output.WriteString(" Current status:\n")
		for i, todo := range todos {
			statusIcon := "○"
			switch todo.Status {
			case TodoStatusInProgress:
				statusIcon = "◐"
			case TodoStatusCompleted:
				statusIcon = "●"
			}
			output.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, statusIcon, todo.Content))
		}
	}

	return NewResult(output.String()), nil
}

// GetTodoList returns the underlying todo list
func (t *TodoWriteTool) GetTodoList() *TodoList {
	return t.todoList
}
