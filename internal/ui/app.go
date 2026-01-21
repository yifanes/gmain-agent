package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NewModel creates a new application model
func NewModel(version, agent, modelName, workDir string) *Model {
	// Initialize textarea
	ta := textarea.New()
	ta.Placeholder = "Send a message... (Enter to send, Alt+Enter for newline)"
	ta.CharLimit = 10000
	ta.ShowLineNumbers = false
	ta.SetHeight(2)
	ta.Focus()

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#58A6FF"))

	// Initialize viewport
	vp := viewport.New(80, 20)
	vp.SetContent("")

	return &Model{
		viewport:     vp,
		textarea:     ta,
		spinner:      sp,
		messages:     make([]Message, 0),
		state:        StateNormal,
		agent:        agent,
		model:        modelName,
		version:      version,
		workDir:      workDir,
		tokens:       TokenStats{MaxTokens: 200000},
		theme:        DefaultTheme(),
		eventChan:    make(chan AgentEvent, 100),
		inputHistory: make([]string, 0),
		historyIndex: -1,
	}
}

// SetSendCallback sets the callback for sending messages
func (m *Model) SetSendCallback(cb func(msg string) error) {
	m.sendCallback = cb
}

// GetEventChannel returns the event channel for agent to send events
func (m *Model) GetEventChannel() chan AgentEvent {
	return m.eventChan
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.waitForAgentEvent(),
	)
}

// waitForAgentEvent waits for events from the agent
func (m *Model) waitForAgentEvent() tea.Cmd {
	return func() tea.Msg {
		event := <-m.eventChan
		return event
	}
}

// tickCmd returns a command that ticks the spinner
func (m *Model) tickCmd() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := m.handleKeyMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)

	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.LineUp(3)
		case tea.MouseButtonWheelDown:
			m.viewport.LineDown(3)
		}

	case spinner.TickMsg:
		if m.state == StateLoading || (m.currentTool != nil && m.currentTool.Status == ToolStatusRunning) {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case AgentEvent:
		cmd := m.handleAgentEvent(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Continue waiting for more events
		cmds = append(cmds, m.waitForAgentEvent())
	}

	// Update textarea if in normal state
	if m.state == StateNormal && !m.isStreaming {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleKeyMsg handles keyboard input
func (m *Model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Clear copy message on any key press
	m.copyMessage = ""

	// Global shortcuts
	switch msg.String() {
	case "ctrl+c":
		if m.state == StateLoading || m.isStreaming {
			// Cancel current operation
			m.state = StateNormal
			m.isStreaming = false
			m.addSystemMessage("Operation cancelled")
			return nil
		}
		m.quitting = true
		return tea.Quit

	case "ctrl+d":
		m.quitting = true
		return tea.Quit

	case "ctrl+l":
		m.messages = nil
		m.updateViewport()
		return nil

	case "ctrl+y":
		// Toggle selection mode (disables mouse capture for text selection)
		m.selectMode = !m.selectMode
		if m.selectMode {
			m.copyMessage = "Selection mode ON - use mouse to select, Ctrl+Y to exit"
			return tea.DisableMouse
		} else {
			m.copyMessage = "Selection mode OFF"
			return tea.EnableMouseCellMotion
		}

	case "?":
		if m.state == StateNormal && !m.textarea.Focused() {
			m.state = StateHelp
			return nil
		}
	}

	// State-specific handling
	switch m.state {
	case StateNormal:
		return m.handleNormalKey(msg)
	case StateConfirm:
		return m.handleConfirmKey(msg)
	case StateHelp:
		if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
			m.state = StateNormal
		}
		return nil
	case StateSelect:
		if msg.String() == "esc" || msg.String() == "ctrl+y" {
			m.selectMode = false
			m.state = StateNormal
		}
		return nil
	}

	return nil
}

// handleNormalKey handles keys in normal state
func (m *Model) handleNormalKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		if msg.Alt {
			// Alt+Enter: insert newline
			return nil
		}
		// Enter: send message
		return m.sendMessage()

	case "up":
		// Check if we're at the first line of input
		if m.textarea.Line() == 0 && len(m.inputHistory) > 0 {
			return m.loadPrevHistory()
		}

	case "down":
		// Check if we're at the last line of input
		if m.textarea.Line() == m.textarea.LineCount()-1 && m.historyIndex >= 0 {
			return m.loadNextHistory()
		}

	case "esc":
		// Clear input or exit
		if m.textarea.Value() != "" {
			m.textarea.Reset()
			return nil
		}

	// Viewport scrolling keys (always work)
	case "pgup", "ctrl+u":
		m.viewport.HalfViewUp()
		return nil
	case "pgdown":
		m.viewport.HalfViewDown()
		return nil
	case "home":
		m.viewport.GotoTop()
		return nil
	case "end":
		m.viewport.GotoBottom()
		return nil

	// Vim-style scrolling (only when textarea is empty to avoid conflicts)
	case "k":
		if m.textarea.Value() == "" {
			m.viewport.LineUp(1)
			return nil
		}
	case "j":
		if m.textarea.Value() == "" {
			m.viewport.LineDown(1)
			return nil
		}
	case "g":
		if m.textarea.Value() == "" {
			m.viewport.GotoTop()
			return nil
		}
	case "G":
		if m.textarea.Value() == "" {
			m.viewport.GotoBottom()
			return nil
		}
	case "c":
		// Copy last assistant response to clipboard
		if m.textarea.Value() == "" {
			m.copyLastResponse()
			return nil
		}
	}

	return nil
}

// handleConfirmKey handles keys in confirm dialog state
func (m *Model) handleConfirmKey(msg tea.KeyMsg) tea.Cmd {
	if m.confirmDialog == nil {
		m.state = StateNormal
		return nil
	}

	switch msg.String() {
	case "left", "h":
		if m.confirmDialog.Selected > 0 {
			m.confirmDialog.Selected--
		}
	case "right", "l":
		if m.confirmDialog.Selected < len(m.confirmDialog.Options)-1 {
			m.confirmDialog.Selected++
		}
	case "enter":
		result := m.confirmDialog.Options[m.confirmDialog.Selected]
		if m.confirmDialog.Callback != nil {
			m.confirmDialog.Callback(result)
		}
		m.confirmDialog = nil
		m.state = StateNormal
	case "esc":
		if m.confirmDialog.Callback != nil {
			m.confirmDialog.Callback("Cancel")
		}
		m.confirmDialog = nil
		m.state = StateNormal
	case "y":
		if m.confirmDialog.Callback != nil {
			m.confirmDialog.Callback("Allow")
		}
		m.confirmDialog = nil
		m.state = StateNormal
	case "n":
		if m.confirmDialog.Callback != nil {
			m.confirmDialog.Callback("Deny")
		}
		m.confirmDialog = nil
		m.state = StateNormal
	case "a":
		if m.confirmDialog.Callback != nil {
			m.confirmDialog.Callback("Allow Always")
		}
		m.confirmDialog = nil
		m.state = StateNormal
	}

	return nil
}

// sendMessage sends the current input to the agent
func (m *Model) sendMessage() tea.Cmd {
	input := m.textarea.Value()
	if input == "" {
		return nil
	}

	// Add to history
	m.inputHistory = append(m.inputHistory, input)
	m.historyIndex = len(m.inputHistory)

	// Add user message
	m.messages = append(m.messages, Message{
		Type:      MessageTypeUser,
		Content:   input,
		Timestamp: time.Now(),
	})

	// Clear input
	m.textarea.Reset()

	// Update viewport
	m.updateViewport()

	// Set loading state
	m.state = StateLoading
	m.isStreaming = true

	// Send to agent
	if m.sendCallback != nil {
		go func() {
			if err := m.sendCallback(input); err != nil {
				m.eventChan <- AgentEvent{
					Type:  AgentEventError,
					Error: err,
				}
			}
		}()
	}

	return m.tickCmd()
}

// loadPrevHistory loads previous history item
func (m *Model) loadPrevHistory() tea.Cmd {
	if len(m.inputHistory) == 0 {
		return nil
	}

	// Save current input if we're starting to navigate history
	if m.historyIndex == len(m.inputHistory) {
		m.savedInput = m.textarea.Value()
	}

	if m.historyIndex > 0 {
		m.historyIndex--
		m.textarea.SetValue(m.inputHistory[m.historyIndex])
	}

	return nil
}

// loadNextHistory loads next history item
func (m *Model) loadNextHistory() tea.Cmd {
	if m.historyIndex < len(m.inputHistory)-1 {
		m.historyIndex++
		m.textarea.SetValue(m.inputHistory[m.historyIndex])
	} else if m.historyIndex == len(m.inputHistory)-1 {
		m.historyIndex = len(m.inputHistory)
		m.textarea.SetValue(m.savedInput)
	}

	return nil
}

// handleWindowSize handles window resize
func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	// Calculate layout
	headerHeight := 1
	statusBarHeight := 1
	inputHeight := 4
	padding := 2

	m.viewportHeight = m.height - headerHeight - statusBarHeight - inputHeight - padding
	if m.viewportHeight < 5 {
		m.viewportHeight = 5
	}

	m.viewport.Width = m.width - 2
	m.viewport.Height = m.viewportHeight
	m.textarea.SetWidth(m.width - 4)

	m.ready = true
	m.updateViewport()
}

// handleAgentEvent handles events from the agent
func (m *Model) handleAgentEvent(event AgentEvent) tea.Cmd {
	switch event.Type {
	case AgentEventText:
		m.streamingText += event.Text
		m.updateStreamingText()
		return nil

	case AgentEventToolStart:
		// First, finalize any pending text as a text block
		m.finalizeStreamingText()

		tool := &ToolExecution{
			ID:        event.ToolID,
			Name:      event.ToolName,
			Input:     event.ToolInput,
			Status:    ToolStatusRunning,
			StartTime: time.Now(),
			Expanded:  true,
		}
		m.currentTool = tool

		// Add tool block to current assistant message
		m.ensureAssistantMessage()
		if len(m.messages) > 0 {
			msg := &m.messages[len(m.messages)-1]
			msg.Blocks = append(msg.Blocks, ContentBlock{
				Type: ContentBlockTool,
				Tool: tool,
			})
		}
		m.updateViewport()
		return m.tickCmd()

	case AgentEventToolEnd:
		if m.currentTool != nil {
			m.currentTool.Status = ToolStatusSuccess
			if event.IsError {
				m.currentTool.Status = ToolStatusError
			}
			m.currentTool.Output = event.ToolOutput
			m.currentTool.EndTime = time.Now()
			m.currentTool.IsError = event.IsError
			m.currentTool = nil
		}
		m.updateViewport()
		return nil

	case AgentEventError:
		m.state = StateNormal
		m.isStreaming = false
		m.addErrorMessage(event.Error.Error())
		return nil

	case AgentEventDone:
		// Finalize any remaining streaming text
		m.finalizeStreamingText()
		m.state = StateNormal
		m.isStreaming = false
		m.updateViewport()
		return nil

	case AgentEventAgentSwitch:
		m.agent = event.Agent
		m.addSystemMessage(fmt.Sprintf("Switched to %s agent", event.Agent))
		return nil

	case AgentEventTokenUpdate:
		m.tokens = event.Tokens
		return nil

	case AgentEventCompaction:
		m.addSystemMessage(event.CompactionInfo)
		return nil

	case AgentEventConfirmRequest:
		if event.ConfirmAction != nil {
			m.confirmDialog = event.ConfirmAction
			m.state = StateConfirm
		}
		return nil
	}

	return nil
}

// addSystemMessage adds a system message
func (m *Model) addSystemMessage(content string) {
	m.messages = append(m.messages, Message{
		Type:      MessageTypeSystem,
		Content:   content,
		Timestamp: time.Now(),
	})
	m.updateViewport()
}

// addErrorMessage adds an error message
func (m *Model) addErrorMessage(content string) {
	m.messages = append(m.messages, Message{
		Type:      MessageTypeError,
		Content:   content,
		Timestamp: time.Now(),
	})
	m.updateViewport()
}

// ensureAssistantMessage ensures there's an assistant message to add content to
func (m *Model) ensureAssistantMessage() {
	if len(m.messages) == 0 || m.messages[len(m.messages)-1].Type != MessageTypeAssistant {
		m.messages = append(m.messages, Message{
			Type:      MessageTypeAssistant,
			Blocks:    make([]ContentBlock, 0),
			Timestamp: time.Now(),
		})
	}
}

// updateStreamingText updates the current streaming text in the message
func (m *Model) updateStreamingText() {
	m.ensureAssistantMessage()

	msg := &m.messages[len(m.messages)-1]

	// If the last block is a text block, update it
	// Otherwise create a new text block (for streaming display)
	if len(msg.Blocks) > 0 && msg.Blocks[len(msg.Blocks)-1].Type == ContentBlockText {
		msg.Blocks[len(msg.Blocks)-1].Text = m.streamingText
	} else {
		// Create a temporary text block for streaming
		msg.Blocks = append(msg.Blocks, ContentBlock{
			Type: ContentBlockText,
			Text: m.streamingText,
		})
	}
	m.updateViewport()
}

// finalizeStreamingText finalizes streaming text as a text block
func (m *Model) finalizeStreamingText() {
	if m.streamingText == "" {
		return
	}

	m.ensureAssistantMessage()

	msg := &m.messages[len(m.messages)-1]

	// If the last block is a text block being streamed, it's already there
	// Just reset the streaming text
	if len(msg.Blocks) > 0 && msg.Blocks[len(msg.Blocks)-1].Type == ContentBlockText {
		msg.Blocks[len(msg.Blocks)-1].Text = m.streamingText
	} else {
		// Add the text as a new block
		msg.Blocks = append(msg.Blocks, ContentBlock{
			Type: ContentBlockText,
			Text: m.streamingText,
		})
	}

	m.streamingText = ""
}

// copyLastResponse copies the last assistant response to clipboard
func (m *Model) copyLastResponse() {
	// Find the last assistant message
	var lastAssistantMsg *Message
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Type == MessageTypeAssistant {
			lastAssistantMsg = &m.messages[i]
			break
		}
	}

	if lastAssistantMsg == nil {
		m.copyMessage = "No response to copy"
		return
	}

	// Build the text content
	var content strings.Builder

	if len(lastAssistantMsg.Blocks) > 0 {
		for _, block := range lastAssistantMsg.Blocks {
			switch block.Type {
			case ContentBlockText:
				if block.Text != "" {
					content.WriteString(block.Text)
					content.WriteString("\n")
				}
			case ContentBlockTool:
				if block.Tool != nil && block.Tool.Output != "" {
					content.WriteString(fmt.Sprintf("\n[%s output]\n%s\n", block.Tool.Name, block.Tool.Output))
				}
			}
		}
	} else if lastAssistantMsg.Content != "" {
		content.WriteString(lastAssistantMsg.Content)
	}

	text := strings.TrimSpace(content.String())
	if text == "" {
		m.copyMessage = "No content to copy"
		return
	}

	if err := clipboard.WriteAll(text); err != nil {
		m.copyMessage = fmt.Sprintf("Copy failed: %v", err)
		return
	}

	// Truncate display message if too long
	displayLen := len(text)
	if displayLen > 50 {
		m.copyMessage = fmt.Sprintf("Copied %d chars to clipboard", displayLen)
	} else {
		m.copyMessage = "Copied to clipboard"
	}
}

// updateViewport updates the viewport content
func (m *Model) updateViewport() {
	content := m.renderMessages()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// View renders the entire UI
func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	return m.renderLayout()
}
