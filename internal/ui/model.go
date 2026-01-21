package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// MessageType represents the type of message
type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAssistant
	MessageTypeSystem
	MessageTypeError
)

// ContentBlockType represents the type of content block
type ContentBlockType int

const (
	ContentBlockText ContentBlockType = iota
	ContentBlockTool
)

// ContentBlock represents a piece of content (text or tool)
type ContentBlock struct {
	Type ContentBlockType
	Text string
	Tool *ToolExecution
}

// Message represents a chat message
type Message struct {
	Type      MessageType
	Content   string           // For simple messages (user, system, error)
	Blocks    []ContentBlock   // For assistant messages with interleaved text/tools
	Timestamp time.Time
	Tools     []ToolExecution  // Deprecated: use Blocks instead
}

// ToolStatus represents the status of a tool execution
type ToolStatus int

const (
	ToolStatusPending ToolStatus = iota
	ToolStatusRunning
	ToolStatusSuccess
	ToolStatusError
)

// ToolExecution represents a tool execution
type ToolExecution struct {
	ID        string
	Name      string
	Input     string
	Output    string
	Status    ToolStatus
	StartTime time.Time
	EndTime   time.Time
	Expanded  bool
	IsError   bool
}

// TokenStats holds token usage statistics
type TokenStats struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	MaxTokens        int
}

// Total returns total tokens used
func (t TokenStats) Total() int {
	return t.InputTokens + t.OutputTokens + t.CacheReadTokens + t.CacheWriteTokens
}

// ConfirmAction represents a permission confirmation action
type ConfirmAction struct {
	Title     string
	Message   string
	Details   string
	ToolName  string
	ToolID    string
	Options   []string
	Selected  int
	Visible   bool
	Callback  func(result string)
}

// AppState represents the current state of the application
type AppState int

const (
	StateNormal AppState = iota
	StateLoading
	StateConfirm
	StateHelp
	StateError
	StateSelect // Selection mode for copying text
)

// Model is the main application model for BubbleTea
type Model struct {
	// Sub-components
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	// Messages
	messages    []Message
	currentTool *ToolExecution

	// State
	state       AppState
	agent       string
	model       string
	version     string
	workDir     string
	tokens      TokenStats
	confirmDialog *ConfirmAction

	// UI state
	width           int
	height          int
	viewportHeight  int
	ready           bool
	streamingText   string
	isStreaming     bool
	selectMode      bool   // Selection mode for copying
	copyMessage     string // Temporary message for copy feedback

	// Input history
	inputHistory []string
	historyIndex int
	savedInput   string

	// Theme
	theme *Theme

	// Channel for agent events
	eventChan chan AgentEvent

	// Callback for sending messages to agent
	sendCallback func(msg string) error

	// Quit signal
	quitting bool
}

// AgentEventType represents types of events from the agent
type AgentEventType int

const (
	AgentEventText AgentEventType = iota
	AgentEventToolStart
	AgentEventToolEnd
	AgentEventError
	AgentEventDone
	AgentEventAgentSwitch
	AgentEventTokenUpdate
	AgentEventCompaction
	AgentEventConfirmRequest
)

// AgentEvent represents an event from the agent
type AgentEvent struct {
	Type           AgentEventType
	Text           string
	ToolName       string
	ToolID         string
	ToolInput      string
	ToolOutput     string
	IsError        bool
	Error          error
	Agent          string
	Tokens         TokenStats
	CompactionInfo string
	ConfirmAction  *ConfirmAction
}

// Theme defines the color scheme
type Theme struct {
	Name string

	// Background and foreground
	Background lipgloss.Color
	Foreground lipgloss.Color

	// Accent colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color

	// Status colors
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	Info    lipgloss.Color

	// Agent colors
	BuildAgent   lipgloss.Color
	PlanAgent    lipgloss.Color
	ExploreAgent lipgloss.Color

	// Border colors
	Border    lipgloss.Color
	BorderDim lipgloss.Color

	// Text colors
	TextPrimary   lipgloss.Color
	TextSecondary lipgloss.Color
	TextDim       lipgloss.Color
}

// DefaultTheme returns the default dark theme
func DefaultTheme() *Theme {
	return &Theme{
		Name:          "dark",
		Background:    lipgloss.Color("#0D1117"),
		Foreground:    lipgloss.Color("#C9D1D9"),
		Primary:       lipgloss.Color("#58A6FF"),
		Secondary:     lipgloss.Color("#8B949E"),
		Accent:        lipgloss.Color("#F78166"),
		Success:       lipgloss.Color("#3FB950"),
		Warning:       lipgloss.Color("#D29922"),
		Error:         lipgloss.Color("#F85149"),
		Info:          lipgloss.Color("#58A6FF"),
		BuildAgent:    lipgloss.Color("#58A6FF"),
		PlanAgent:     lipgloss.Color("#A371F7"),
		ExploreAgent:  lipgloss.Color("#3FB950"),
		Border:        lipgloss.Color("#30363D"),
		BorderDim:     lipgloss.Color("#21262D"),
		TextPrimary:   lipgloss.Color("#C9D1D9"),
		TextSecondary: lipgloss.Color("#8B949E"),
		TextDim:       lipgloss.Color("#484F58"),
	}
}
