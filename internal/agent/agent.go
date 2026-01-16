package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/anthropics/claude-code-go/internal/agentregistry"
	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/compaction"
	"github.com/anthropics/claude-code-go/internal/logger"
	"github.com/anthropics/claude-code-go/internal/permission"
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
	EventTypeAgentSwitch    EventType = "agent_switch"
	EventTypeCompaction     EventType = "compaction"
	EventTypeTokenUsage     EventType = "token_usage"
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
	AgentName  string // For agent switch events

	// Token usage
	TokenUsage *api.Usage

	// Compaction info
	CompactionInfo string
}

// EventHandler is a function that handles events
type EventHandler func(event Event)

// Agent represents the main Claude agent
type Agent struct {
	client        *api.Client
	registry      *tools.Registry
	agentRegistry *agentregistry.Registry
	permEvaluator *permission.Evaluator
	compactor     *compaction.Compactor
	conversation  *Conversation
	eventHandler  EventHandler
	workDir       string
	currentAgent  string // Current agent name (build, plan, explore)
	sessionID     string // Session ID for output truncation

	// Token tracking
	totalInputTokens      int
	totalOutputTokens     int
	totalCacheReadTokens  int
	totalCacheWriteTokens int
}

// NewAgent creates a new agent
func NewAgent(client *api.Client, registry *tools.Registry, agentRegistry *agentregistry.Registry, workDir string) *Agent {
	// Get build agent info for initial system prompt
	buildAgent, _ := agentRegistry.Get("build")
	systemPrompt := buildAgent.GetSystemPrompt(workDir)

	// Generate session ID
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())

	return &Agent{
		client:        client,
		registry:      registry,
		agentRegistry: agentRegistry,
		permEvaluator: permission.NewEvaluator(),
		compactor:     compaction.NewCompactor(client),
		conversation:  NewConversation(systemPrompt),
		workDir:       workDir,
		currentAgent:  "build", // Start with build agent
		sessionID:     sessionID,
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

// GetCurrentAgent returns the current agent name
func (a *Agent) GetCurrentAgent() string {
	return a.currentAgent
}

// GetTokenUsage returns the total token usage
func (a *Agent) GetTokenUsage() (input, output, cacheRead, cacheWrite int) {
	return a.totalInputTokens, a.totalOutputTokens, a.totalCacheReadTokens, a.totalCacheWriteTokens
}

// trackTokens tracks token usage from a response
func (a *Agent) trackTokens(usage api.Usage) {
	a.totalInputTokens += usage.InputTokens
	a.totalOutputTokens += usage.OutputTokens
	a.totalCacheReadTokens += usage.CacheReadInputTokens
	a.totalCacheWriteTokens += usage.CacheCreationInputTokens

	// Emit token usage event
	a.emit(Event{
		Type:       EventTypeTokenUsage,
		TokenUsage: &usage,
	})
}

// SwitchAgent switches to a different agent
func (a *Agent) SwitchAgent(agentName string) error {
	// Get new agent info
	newAgent, err := a.agentRegistry.Get(agentName)
	if err != nil {
		return fmt.Errorf("failed to get agent %s: %w", agentName, err)
	}

	// Update current agent
	a.currentAgent = agentName

	// Update system prompt
	systemPrompt := newAgent.GetSystemPrompt(a.workDir)
	a.conversation.SetSystemMessage(systemPrompt)

	// Emit agent switch event
	a.emit(Event{
		Type:      EventTypeAgentSwitch,
		AgentName: agentName,
	})

	return nil
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

		// Track token usage from stream response
		streamResp := stream.GetResponse()
		if streamResp != nil {
			a.trackTokens(streamResp.Usage)
		}

		stream.Close()

		if err != nil {
			a.emit(Event{Type: EventTypeError, Error: err})
			return fmt.Errorf("failed to process stream: %w", err)
		}

		// Add assistant response to conversation
		if len(content) > 0 {
			a.conversation.AddAssistantMessage(content)
		}

		// Check if compaction is needed
		if err := a.checkAndCompact(ctx); err != nil {
			// Log error but continue
			if log := logger.GetLogger(); log != nil {
				log.LogError("compaction_error", err, map[string]interface{}{
					"session_id": a.sessionID,
				})
			}
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

	// Get current agent permissions
	agentInfo, err := a.agentRegistry.Get(a.currentAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current agent: %w", err)
	}

	for _, call := range toolCalls {
		if call.Type != api.ContentTypeToolUse {
			continue
		}

		// Log tool call
		if log := logger.GetLogger(); log != nil {
			var inputMap map[string]interface{}
			json.Unmarshal(call.Input, &inputMap)
			log.LogToolCall(call.Name, call.ID, inputMap)
		}

		// Check permissions before execution
		var inputMap map[string]interface{}
		json.Unmarshal(call.Input, &inputMap)

		// Extract pattern from input for permission check
		pattern := extractPattern(call.Name, inputMap)
		action := a.permEvaluator.Evaluate(call.Name, pattern, agentInfo.Permission)

		// Handle permission denial
		if action == permission.ActionDeny {
			output := fmt.Sprintf("Permission denied: agent '%s' is not allowed to use tool '%s' with pattern '%s'",
				a.currentAgent, call.Name, pattern)

			a.emit(Event{
				Type:       EventTypeToolUseEnd,
				ToolName:   call.Name,
				ToolID:     call.ID,
				ToolResult: output,
				IsError:    true,
			})

			results = append(results, api.Content{
				Type:      api.ContentTypeToolResult,
				ToolUseID: call.ID,
				Content:   output,
				IsError:   true,
			})
			continue
		}

		// Execute the tool
		startTime := time.Now()
		result, err := a.registry.Execute(ctx, call.Name, call.Input)
		duration := time.Since(startTime)

		var output string
		var isError bool

		if err != nil {
			output = err.Error()
			isError = true
		} else {
			output = result.Output
			isError = result.IsError
		}

		// Apply output truncation if needed
		output = a.truncateOutput(output, call.Name, call.ID)

		// Log tool result
		if log := logger.GetLogger(); log != nil {
			log.LogToolResult(call.Name, call.ID, output, isError, duration)
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

// extractPattern extracts the pattern from tool input for permission checking
func extractPattern(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "read", "write", "edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "bash":
		if cmd, ok := input["command"].(string); ok {
			return cmd
		}
	case "glob":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "grep":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	}
	return "*"
}

// checkAndCompact checks if compaction is needed and performs it
func (a *Agent) checkAndCompact(ctx context.Context) error {
	// Calculate current token usage
	usage := compaction.TokenUsage{
		Input:     a.totalInputTokens,
		Output:    a.totalOutputTokens,
		CacheRead: a.totalCacheReadTokens,
	}

	limits := compaction.DefaultModelLimits()

	// Check if we need compaction (80% threshold)
	if !compaction.NeedsCompaction(usage, limits) {
		return nil
	}

	// Emit compaction start event
	a.emit(Event{
		Type:           EventTypeCompaction,
		CompactionInfo: "Starting conversation compaction...",
	})

	// Try pruning first (faster than full compaction)
	messages := a.conversation.GetMessages()
	if compaction.CanPrune(messages) {
		pruneResult := compaction.Prune(messages)
		if pruneResult.PrunedCount > 0 {
			// Replace messages with pruned version
			a.conversation.Clear()
			for _, msg := range pruneResult.Messages {
				a.conversation.AddMessage(msg)
			}

			info := fmt.Sprintf("Pruned %d tool results (%d chars)", pruneResult.PrunedCount, pruneResult.PrunedChars)
			a.emit(Event{
				Type:           EventTypeCompaction,
				CompactionInfo: info,
			})
			return nil
		}
	}

	// If pruning not enough, do full compaction
	compactResult, err := a.compactor.Compact(ctx, compaction.CompactInput{
		Messages:   messages,
		Model:      a.client.GetModel(),
		MaxTokens:  4000,
		KeepRecent: 2,
	})
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	// Replace conversation with compacted version
	a.conversation.Clear()
	for _, msg := range compactResult.Messages {
		a.conversation.AddMessage(msg)
	}

	info := fmt.Sprintf("Compacted %d messages into summary", compactResult.CompactedCount)
	a.emit(Event{
		Type:           EventTypeCompaction,
		CompactionInfo: info,
	})

	return nil
}

// truncateOutput truncates tool output if needed
func (a *Agent) truncateOutput(output string, toolName string, callID string) string {
	result := compaction.TruncateOutput(output, a.sessionID, toolName, callID)
	if result.Truncated {
		return result.Content
	}
	return output
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

IMPORTANT - Long-Running Processes:
For processes that run indefinitely (dev servers, watch tasks, daemons):
- Set "run_in_background": true in Bash tool parameters, OR
- Append & to the command (e.g., "npm run dev &")
The process will run in background and return immediately with PID and log file path.

Examples:
  {"command": "npm run dev", "run_in_background": true}  // Background server
  {"command": "npm run dev &"}                            // Alternative syntax
  {"command": "npm install"}                              // Normal command

Timeouts:
- Default timeout: 15 seconds (not 2 minutes anymore!)
- Max timeout: 2 minutes
- Background commands: 5 seconds to start

Remember to use the TodoWrite tool to track complex multi-step tasks.`, workDir)
}
