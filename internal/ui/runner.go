package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIRunner manages the TUI application
type TUIRunner struct {
	model   *Model
	program *tea.Program
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewTUIRunner creates a new TUI runner
func NewTUIRunner(version, agent, modelName, workDir string) *TUIRunner {
	ctx, cancel := context.WithCancel(context.Background())
	model := NewModel(version, agent, modelName, workDir)

	return &TUIRunner{
		model:  model,
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetSendCallback sets the callback for sending messages to the agent
func (r *TUIRunner) SetSendCallback(cb func(msg string) error) {
	r.model.SetSendCallback(cb)
}

// GetEventChannel returns the event channel for sending events from the agent
func (r *TUIRunner) GetEventChannel() chan AgentEvent {
	return r.model.GetEventChannel()
}

// Run starts the TUI application
func (r *TUIRunner) Run() error {
	r.program = tea.NewProgram(
		r.model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := r.program.Run()
	return err
}

// Stop stops the TUI application
func (r *TUIRunner) Stop() {
	r.cancel()
	if r.program != nil {
		r.program.Quit()
	}
}

// SendEvent sends an event to the TUI
func (r *TUIRunner) SendEvent(event AgentEvent) {
	if r.model != nil && r.model.eventChan != nil {
		select {
		case r.model.eventChan <- event:
		default:
			// Channel full, skip event
		}
	}
}

// UpdateTokens updates the token statistics
func (r *TUIRunner) UpdateTokens(input, output, cacheRead, cacheWrite int) {
	r.SendEvent(AgentEvent{
		Type: AgentEventTokenUpdate,
		Tokens: TokenStats{
			InputTokens:      input,
			OutputTokens:     output,
			CacheReadTokens:  cacheRead,
			CacheWriteTokens: cacheWrite,
			MaxTokens:        200000,
		},
	})
}

// AgentEventAdapter adapts agent events to TUI events
type AgentEventAdapter struct {
	eventChan chan AgentEvent
}

// NewAgentEventAdapter creates a new adapter
func NewAgentEventAdapter(eventChan chan AgentEvent) *AgentEventAdapter {
	return &AgentEventAdapter{
		eventChan: eventChan,
	}
}

// OnText handles text streaming events
func (a *AgentEventAdapter) OnText(text string) {
	a.eventChan <- AgentEvent{
		Type: AgentEventText,
		Text: text,
	}
}

// OnToolStart handles tool start events
func (a *AgentEventAdapter) OnToolStart(name, id, input string) {
	a.eventChan <- AgentEvent{
		Type:      AgentEventToolStart,
		ToolName:  name,
		ToolID:    id,
		ToolInput: input,
	}
}

// OnToolEnd handles tool end events
func (a *AgentEventAdapter) OnToolEnd(name, id, output string, isError bool) {
	a.eventChan <- AgentEvent{
		Type:       AgentEventToolEnd,
		ToolName:   name,
		ToolID:     id,
		ToolOutput: output,
		IsError:    isError,
	}
}

// OnError handles error events
func (a *AgentEventAdapter) OnError(err error) {
	a.eventChan <- AgentEvent{
		Type:  AgentEventError,
		Error: err,
	}
}

// OnDone handles completion events
func (a *AgentEventAdapter) OnDone() {
	a.eventChan <- AgentEvent{
		Type: AgentEventDone,
	}
}

// OnAgentSwitch handles agent switch events
func (a *AgentEventAdapter) OnAgentSwitch(agent string) {
	a.eventChan <- AgentEvent{
		Type:  AgentEventAgentSwitch,
		Agent: agent,
	}
}

// OnTokenUpdate handles token update events
func (a *AgentEventAdapter) OnTokenUpdate(input, output, cacheRead, cacheWrite int) {
	a.eventChan <- AgentEvent{
		Type: AgentEventTokenUpdate,
		Tokens: TokenStats{
			InputTokens:      input,
			OutputTokens:     output,
			CacheReadTokens:  cacheRead,
			CacheWriteTokens: cacheWrite,
			MaxTokens:        200000,
		},
	}
}

// OnCompaction handles compaction events
func (a *AgentEventAdapter) OnCompaction(info string) {
	a.eventChan <- AgentEvent{
		Type:           AgentEventCompaction,
		CompactionInfo: info,
	}
}

// OnConfirmRequest handles permission confirmation requests
func (a *AgentEventAdapter) OnConfirmRequest(title, message, details string, callback func(string)) {
	a.eventChan <- AgentEvent{
		Type: AgentEventConfirmRequest,
		ConfirmAction: &ConfirmAction{
			Title:    title,
			Message:  message,
			Details:  details,
			Options:  []string{"Allow", "Deny", "Allow Always"},
			Callback: callback,
		},
	}
}

// SimpleTUI provides a simple interface for running the TUI without full integration
type SimpleTUI struct {
	runner  *TUIRunner
	adapter *AgentEventAdapter
}

// NewSimpleTUI creates a new simple TUI
func NewSimpleTUI(version, agent, modelName, workDir string) *SimpleTUI {
	runner := NewTUIRunner(version, agent, modelName, workDir)
	adapter := NewAgentEventAdapter(runner.GetEventChannel())

	return &SimpleTUI{
		runner:  runner,
		adapter: adapter,
	}
}

// SetMessageHandler sets the handler for user messages
func (s *SimpleTUI) SetMessageHandler(handler func(msg string) error) {
	s.runner.SetSendCallback(handler)
}

// GetAdapter returns the event adapter
func (s *SimpleTUI) GetAdapter() *AgentEventAdapter {
	return s.adapter
}

// Run starts the TUI
func (s *SimpleTUI) Run() error {
	return s.runner.Run()
}

// Stop stops the TUI
func (s *SimpleTUI) Stop() {
	s.runner.Stop()
}

// PrintWelcome is a compatibility method (does nothing in TUI mode)
func (s *SimpleTUI) PrintWelcome() {
	// TUI handles welcome internally
}

// PrintInfo sends an info message to the TUI
func (s *SimpleTUI) PrintInfo(msg string) {
	s.adapter.eventChan <- AgentEvent{
		Type:           AgentEventCompaction,
		CompactionInfo: msg,
	}
}

// PrintError sends an error message to the TUI
func (s *SimpleTUI) PrintError(err error) {
	s.adapter.OnError(err)
}

// PrintSuccess sends a success message to the TUI
func (s *SimpleTUI) PrintSuccess(msg string) {
	s.adapter.eventChan <- AgentEvent{
		Type:           AgentEventCompaction,
		CompactionInfo: fmt.Sprintf("✓ %s", msg),
	}
}
