package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/tools"
)

// EventType represents the type of event emitted by the agent
type EventType string

const (
	EventTypeText           EventType = "text"
	EventTypeToolUseStart   EventType = "tool_use_start"
	EventTypeToolUseEnd     EventType = "tool_use_end"
	EventTypeThinking       EventType = "thinking"
	EventTypeError          EventType = "error"
	EventTypeConversationEnd EventType = "conversation_end"
)

// Event represents an event emitted during agent execution
type Event struct {
	Type       EventType
	Text       string
	ToolName   string
	ToolID     string
	ToolInput  string
	ToolResult string
	IsError    bool
	Error      error
}

// EventHandler is a function that handles events
type EventHandler func(event Event)

// Agent represents the main Claude agent
type Agent struct {
	client       *api.Client
	registry     *tools.Registry
	conversation *Conversation
	eventHandler EventHandler
	workDir      string
}

// NewAgent creates a new agent
func NewAgent(client *api.Client, registry *tools.Registry, workDir string) *Agent {
	return &Agent{
		client:       client,
		registry:     registry,
		conversation: NewConversation(DefaultSystemPrompt(workDir)),
		workDir:      workDir,
	}
}

// SetEventHandler sets the event handler for the agent
func (a *Agent) SetEventHandler(handler EventHandler) {
	a.eventHandler = handler
}

// SetSystemPrompt sets a custom system prompt
func (a *Agent) SetSystemPrompt(prompt string) {
	a.conversation.SetSystemMessage(prompt)
}

// GetConversation returns the conversation
func (a *Agent) GetConversation() *Conversation {
	return a.conversation
}

// emit emits an event to the handler
func (a *Agent) emit(event Event) {
	if a.eventHandler != nil {
		a.eventHandler(event)
	}
}

// Chat sends a user message and processes the response
func (a *Agent) Chat(ctx context.Context, userMessage string) error {
	// Add user message to conversation
	a.conversation.AddUserMessage(userMessage)

	// Run the agent loop
	return a.runLoop(ctx)
}

// runLoop runs the main agent loop until no more tool calls
func (a *Agent) runLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Build request
		req := &api.MessagesRequest{
			System:   a.conversation.GetSystemMessage(),
			Messages: a.conversation.GetMessages(),
			Tools:    a.registry.ToAPITools(),
		}

		// Stream the response
		stream, err := a.client.StreamMessage(ctx, req)
		if err != nil {
			a.emit(Event{Type: EventTypeError, Error: err})
			return fmt.Errorf("failed to send message: %w", err)
		}

		// Process stream and collect response
		content, toolCalls, err := a.processStream(ctx, stream)
		stream.Close()

		if err != nil {
			a.emit(Event{Type: EventTypeError, Error: err})
			return fmt.Errorf("failed to process stream: %w", err)
		}

		// Add assistant response to conversation
		if len(content) > 0 {
			a.conversation.AddAssistantMessage(content)
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			a.emit(Event{Type: EventTypeConversationEnd})
			return nil
		}

		// Execute tool calls
		toolResults, err := a.executeToolCalls(ctx, toolCalls)
		if err != nil {
			return fmt.Errorf("failed to execute tools: %w", err)
		}

		// Add tool results to conversation
		a.conversation.AddToolResults(toolResults)
	}
}

// processStream processes the streaming response
func (a *Agent) processStream(ctx context.Context, stream *api.StreamReader) ([]api.Content, []api.Content, error) {
	var content []api.Content
	var toolCalls []api.Content
	var currentText strings.Builder
	var currentToolInput strings.Builder
	var currentToolIndex int = -1

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		switch chunk.Type {
		case "text":
			currentText.WriteString(chunk.Text)
			a.emit(Event{Type: EventTypeText, Text: chunk.Text})

		case "tool_use_start":
			// Finalize any pending text
			if currentText.Len() > 0 {
				content = append(content, api.Content{
					Type: api.ContentTypeText,
					Text: currentText.String(),
				})
				currentText.Reset()
			}

			currentToolIndex = chunk.Index
			currentToolInput.Reset()

			a.emit(Event{
				Type:     EventTypeToolUseStart,
				ToolName: chunk.ContentBlock.Name,
				ToolID:   chunk.ContentBlock.ID,
			})

		case "tool_use_delta":
			currentToolInput.WriteString(chunk.PartialJSON)

		case "content_block_stop":
			if currentToolIndex >= 0 && chunk.Index == currentToolIndex {
				// Get the content block from the stream response
				resp := stream.GetResponse()
				if chunk.Index < len(resp.Content) {
					block := resp.Content[chunk.Index]
					if block.Type == api.ContentTypeToolUse {
						// Parse the accumulated input
						var input json.RawMessage
						if currentToolInput.Len() > 0 {
							input = json.RawMessage(currentToolInput.String())
						} else {
							input = block.Input
						}

						toolCall := api.Content{
							Type:  api.ContentTypeToolUse,
							ID:    block.ID,
							Name:  block.Name,
							Input: input,
						}
						toolCalls = append(toolCalls, toolCall)
						content = append(content, toolCall)
					}
				}
				currentToolIndex = -1
			}

		case "message_stop":
			// Finalize any pending text
			if currentText.Len() > 0 {
				content = append(content, api.Content{
					Type: api.ContentTypeText,
					Text: currentText.String(),
				})
			}

		case "error":
			return nil, nil, chunk.Error
		}
	}

	return content, toolCalls, nil
}

// executeToolCalls executes all tool calls and returns results
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []api.Content) ([]api.Content, error) {
	var results []api.Content

	for _, call := range toolCalls {
		if call.Type != api.ContentTypeToolUse {
			continue
		}

		// Execute the tool
		result, err := a.registry.Execute(ctx, call.Name, call.Input)

		var output string
		var isError bool

		if err != nil {
			output = err.Error()
			isError = true
		} else {
			output = result.Output
			isError = result.IsError
		}

		a.emit(Event{
			Type:       EventTypeToolUseEnd,
			ToolName:   call.Name,
			ToolID:     call.ID,
			ToolResult: output,
			IsError:    isError,
		})

		results = append(results, api.Content{
			Type:      api.ContentTypeToolResult,
			ToolUseID: call.ID,
			Content:   output,
			IsError:   isError,
		})
	}

	return results, nil
}

// DefaultSystemPrompt returns the default system prompt
func DefaultSystemPrompt(workDir string) string {
	return fmt.Sprintf(`You are Claude, an AI assistant created by Anthropic. You are helping the user with software engineering tasks through a CLI tool called Claude Code.

Working Directory: %s

You have access to various tools to help with tasks:
- Bash: Execute shell commands
- Read: Read file contents
- Write: Write files
- Edit: Edit files using string replacement
- Glob: Find files by pattern
- Grep: Search file contents
- WebFetch: Fetch web content
- TodoWrite: Manage task lists
- AskUserQuestion: Ask the user questions

Guidelines:
1. Always read files before editing them
2. Use absolute paths when possible
3. Be concise and focused in your responses
4. Complete tasks thoroughly before moving on
5. Use tools proactively to accomplish tasks
6. When editing files, ensure the old_string is unique or use replace_all

Remember to use the TodoWrite tool to track complex multi-step tasks.`, workDir)
}
