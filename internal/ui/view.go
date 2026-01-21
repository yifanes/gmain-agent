package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#C9D1D9")).
			Background(lipgloss.Color("#161B22")).
			Padding(0, 1)

	// Message styles
	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#58A6FF"))

	assistantLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#3FB950"))

	systemMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8B949E")).
				Italic(true)

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F85149")).
				Bold(true)

	// Tool styles
	toolHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C9D1D9"))

	toolInputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E")).
			MarginLeft(2)

	toolOutputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E")).
			MarginLeft(2)

	toolBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#30363D")).
			Padding(0, 1).
			MarginLeft(2)

	// Status bar styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E")).
			Background(lipgloss.Color("#161B22")).
			Padding(0, 1)

	// Input area styles
	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#30363D")).
				Padding(0, 1)

	// Dialog styles
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#D29922")).
			Padding(1, 2)

	dialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#D29922"))

	dialogButtonStyle = lipgloss.NewStyle().
				Padding(0, 2).
				MarginRight(1).
				Background(lipgloss.Color("#30363D")).
				Foreground(lipgloss.Color("#8B949E"))

	dialogButtonSelectedStyle = lipgloss.NewStyle().
					Padding(0, 2).
					MarginRight(1).
					Background(lipgloss.Color("#58A6FF")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Bold(true)

	// Help styles
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#58A6FF")).
			Width(12)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#484F58"))
)

// renderLayout renders the main layout
func (m *Model) renderLayout() string {
	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Message area (viewport)
	sections = append(sections, m.viewport.View())

	// Confirm dialog (if visible)
	if m.state == StateConfirm && m.confirmDialog != nil {
		sections = append(sections, m.renderConfirmDialog())
	}

	// Help panel (if visible)
	if m.state == StateHelp {
		sections = append(sections, m.renderHelpPanel())
	}

	// Input area
	sections = append(sections, m.renderInputArea())

	// Status bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header
func (m *Model) renderHeader() string {
	// Left: project name and version
	left := fmt.Sprintf("gmain-agent v%s", m.version)

	// Center: model name
	center := m.model

	// Right: current time
	right := time.Now().Format("15:04:05")

	// Calculate widths
	leftWidth := m.width / 3
	centerWidth := m.width / 3
	rightWidth := m.width - leftWidth - centerWidth

	leftStyled := lipgloss.NewStyle().
		Width(leftWidth).
		Align(lipgloss.Left).
		Render(left)

	centerStyled := lipgloss.NewStyle().
		Width(centerWidth).
		Align(lipgloss.Center).
		Render(center)

	rightStyled := lipgloss.NewStyle().
		Width(rightWidth).
		Align(lipgloss.Right).
		Render(right)

	header := lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, centerStyled, rightStyled)
	return headerStyle.Width(m.width).Render(header)
}

// renderMessages renders all messages
func (m *Model) renderMessages() string {
	var parts []string

	for _, msg := range m.messages {
		parts = append(parts, m.renderMessage(msg))
	}

	// Add streaming indicator if streaming and no content yet
	if m.isStreaming && m.state == StateLoading {
		// Only show "Thinking..." if we have no messages or no blocks yet
		hasContent := false
		if len(m.messages) > 0 {
			lastMsg := m.messages[len(m.messages)-1]
			if lastMsg.Type == MessageTypeAssistant && len(lastMsg.Blocks) > 0 {
				hasContent = true
			}
		}
		if !hasContent {
			parts = append(parts, m.spinner.View()+" Thinking...")
		}
	}

	return strings.Join(parts, "\n\n")
}

// renderMessage renders a single message
func (m *Model) renderMessage(msg Message) string {
	var parts []string

	switch msg.Type {
	case MessageTypeUser:
		label := userLabelStyle.Render("You:")
		parts = append(parts, label+" "+msg.Content)

	case MessageTypeAssistant:
		label := assistantLabelStyle.Render("Claude:")
		parts = append(parts, label)

		// Render content blocks in order (new approach)
		if len(msg.Blocks) > 0 {
			for _, block := range msg.Blocks {
				switch block.Type {
				case ContentBlockText:
					if block.Text != "" {
						lines := strings.Split(block.Text, "\n")
						for _, line := range lines {
							parts = append(parts, "  "+line)
						}
					}
				case ContentBlockTool:
					if block.Tool != nil {
						parts = append(parts, m.renderToolBlock(*block.Tool))
					}
				}
			}
		} else if msg.Content != "" {
			// Fallback for old-style messages
			lines := strings.Split(msg.Content, "\n")
			for _, line := range lines {
				parts = append(parts, "  "+line)
			}
			// Render tools (old approach)
			for _, tool := range msg.Tools {
				parts = append(parts, m.renderToolBlock(tool))
			}
		}

	case MessageTypeSystem:
		parts = append(parts, systemMessageStyle.Render("  "+msg.Content))

	case MessageTypeError:
		parts = append(parts, errorMessageStyle.Render("Error: "+msg.Content))
	}

	return strings.Join(parts, "\n")
}

// renderToolBlock renders a tool execution block
func (m *Model) renderToolBlock(tool ToolExecution) string {
	var parts []string

	// Header: status icon + name + duration
	var icon string
	var iconColor lipgloss.Color

	switch tool.Status {
	case ToolStatusPending:
		icon = "○"
		iconColor = lipgloss.Color("#8B949E")
	case ToolStatusRunning:
		icon = m.spinner.View()
		iconColor = lipgloss.Color("#58A6FF")
	case ToolStatusSuccess:
		icon = "✓"
		iconColor = lipgloss.Color("#3FB950")
	case ToolStatusError:
		icon = "✗"
		iconColor = lipgloss.Color("#F85149")
	}

	iconStyled := lipgloss.NewStyle().Foreground(iconColor).Render(icon)

	// Expand/collapse indicator
	expandIcon := "▶"
	if tool.Expanded {
		expandIcon = "▼"
	}

	// Duration
	var duration string
	if tool.Status == ToolStatusRunning {
		duration = fmt.Sprintf("(%.1fs...)", time.Since(tool.StartTime).Seconds())
	} else if !tool.EndTime.IsZero() {
		duration = fmt.Sprintf("(%.1fs)", tool.EndTime.Sub(tool.StartTime).Seconds())
	}

	header := fmt.Sprintf("  %s %s %s %s",
		dimStyle.Render(expandIcon),
		iconStyled,
		toolHeaderStyle.Bold(true).Render(tool.Name),
		dimStyle.Render(duration),
	)
	parts = append(parts, header)

	// Details (if expanded)
	if tool.Expanded {
		// Input
		if tool.Input != "" {
			inputLabel := dimStyle.Render("    Input:")
			parts = append(parts, inputLabel)
			// Truncate long input
			input := tool.Input
			if len(input) > 200 {
				input = input[:200] + "..."
			}
			parts = append(parts, toolInputStyle.Render("    "+input))
		}

		// Output
		if tool.Output != "" {
			var outputLabel string
			if tool.IsError {
				outputLabel = errorMessageStyle.Render("    Error:")
			} else {
				outputLabel = dimStyle.Render("    Output:")
			}
			parts = append(parts, outputLabel)

			// Truncate long output
			output := tool.Output
			lines := strings.Split(output, "\n")
			maxLines := 10
			if len(lines) > maxLines {
				lines = lines[:maxLines]
				lines = append(lines, fmt.Sprintf("... (%d more lines)", len(strings.Split(output, "\n"))-maxLines))
			}
			for _, line := range lines {
				if len(line) > m.width-10 {
					line = line[:m.width-10] + "..."
				}
				parts = append(parts, toolOutputStyle.Render("    "+line))
			}
		}
	}

	return strings.Join(parts, "\n")
}

// renderInputArea renders the input area
func (m *Model) renderInputArea() string {
	// Prompt indicator
	var prompt string
	if m.state == StateLoading {
		prompt = m.spinner.View() + " "
	} else {
		prompt = "> "
	}

	// Input textarea
	input := m.textarea.View()

	// Combine
	content := prompt + input

	return inputBorderStyle.Width(m.width - 2).Render(content)
}

// renderStatusBar renders the status bar
func (m *Model) renderStatusBar() string {
	// Left: Token info or copy message
	var leftContent string
	if m.copyMessage != "" {
		leftContent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3FB950")).
			Render(m.copyMessage)
	} else {
		tokenInfo := fmt.Sprintf("Tokens: %s/%s",
			formatTokenCount(m.tokens.Total()),
			formatTokenCount(m.tokens.MaxTokens),
		)
		if m.tokens.CacheReadTokens > 0 {
			tokenInfo += fmt.Sprintf(" (+%s cache)", formatTokenCount(m.tokens.CacheReadTokens))
		}
		leftContent = tokenInfo
	}

	// Center: Hints
	var hints string
	if m.selectMode {
		hints = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D29922")).
			Render("SELECT MODE: Use mouse to select text | Ctrl+Y to exit")
	} else if m.state == StateConfirm {
		hints = "← → Select | Enter Confirm | y Allow | n Deny | Esc Cancel"
	} else {
		hints = "Enter Send | c Copy | Ctrl+Y Select | ? Help"
	}

	// Right: Agent badge
	agentBadge := m.renderAgentBadge()

	// Calculate widths
	leftWidth := m.width / 4
	centerWidth := m.width / 2
	rightWidth := m.width - leftWidth - centerWidth

	leftStyled := lipgloss.NewStyle().
		Width(leftWidth).
		Align(lipgloss.Left).
		Render(leftContent)

	centerStyled := lipgloss.NewStyle().
		Width(centerWidth).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("#8B949E")).
		Render(hints)

	rightStyled := lipgloss.NewStyle().
		Width(rightWidth).
		Align(lipgloss.Right).
		Render(agentBadge)

	bar := lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, centerStyled, rightStyled)
	return statusBarStyle.Width(m.width).Render(bar)
}

// renderAgentBadge renders the agent badge
func (m *Model) renderAgentBadge() string {
	var bgColor lipgloss.Color

	switch m.agent {
	case "build":
		bgColor = lipgloss.Color("#58A6FF")
	case "plan":
		bgColor = lipgloss.Color("#A371F7")
	case "explore":
		bgColor = lipgloss.Color("#3FB950")
	default:
		bgColor = lipgloss.Color("#8B949E")
	}

	return lipgloss.NewStyle().
		Background(bgColor).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Bold(true).
		Render(m.agent)
}

// renderConfirmDialog renders the permission confirmation dialog
func (m *Model) renderConfirmDialog() string {
	if m.confirmDialog == nil {
		return ""
	}

	var parts []string

	// Title
	title := dialogTitleStyle.Render("⚠ " + m.confirmDialog.Title)
	parts = append(parts, title)
	parts = append(parts, "")

	// Message
	parts = append(parts, m.confirmDialog.Message)
	parts = append(parts, "")

	// Details (command/path)
	if m.confirmDialog.Details != "" {
		detailBox := lipgloss.NewStyle().
			Background(lipgloss.Color("#21262D")).
			Foreground(lipgloss.Color("#C9D1D9")).
			Padding(0, 1).
			Render(m.confirmDialog.Details)
		parts = append(parts, detailBox)
		parts = append(parts, "")
	}

	// Buttons
	var buttons []string
	for i, opt := range m.confirmDialog.Options {
		var btn string
		if i == m.confirmDialog.Selected {
			btn = dialogButtonSelectedStyle.Render(opt)
		} else {
			btn = dialogButtonStyle.Render(opt)
		}
		buttons = append(buttons, btn)
	}
	buttonRow := lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
	parts = append(parts, buttonRow)
	parts = append(parts, "")

	// Hints
	hints := dimStyle.Render("y Allow | n Deny | a Always | Esc Cancel")
	parts = append(parts, hints)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Center the dialog
	dialogWidth := min(m.width-4, 60)
	dialog := dialogStyle.Width(dialogWidth).Render(content)

	return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, dialog)
}

// renderHelpPanel renders the help panel
func (m *Model) renderHelpPanel() string {
	var parts []string

	parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF")).Render("Keyboard Shortcuts"))
	parts = append(parts, "")

	// Global
	parts = append(parts, lipgloss.NewStyle().Bold(true).Render("Global"))
	parts = append(parts, renderHelpItem("Ctrl+C", "Cancel / Quit"))
	parts = append(parts, renderHelpItem("Ctrl+L", "Clear screen"))
	parts = append(parts, renderHelpItem("Ctrl+D", "Exit"))
	parts = append(parts, renderHelpItem("?", "Toggle help"))
	parts = append(parts, "")

	// Scrolling
	parts = append(parts, lipgloss.NewStyle().Bold(true).Render("Scrolling"))
	parts = append(parts, renderHelpItem("j / k", "Scroll down / up"))
	parts = append(parts, renderHelpItem("PgDn/PgUp", "Half page down / up"))
	parts = append(parts, renderHelpItem("Ctrl+D/U", "Half page down / up"))
	parts = append(parts, renderHelpItem("g / G", "Go to top / bottom"))
	parts = append(parts, renderHelpItem("Mouse", "Scroll wheel"))
	parts = append(parts, "")

	// Input
	parts = append(parts, lipgloss.NewStyle().Bold(true).Render("Input"))
	parts = append(parts, renderHelpItem("Enter", "Send message"))
	parts = append(parts, renderHelpItem("Alt+Enter", "New line"))
	parts = append(parts, renderHelpItem("Up/Down", "History navigation"))
	parts = append(parts, renderHelpItem("Esc", "Clear input"))
	parts = append(parts, "")

	// Copy
	parts = append(parts, lipgloss.NewStyle().Bold(true).Render("Copy"))
	parts = append(parts, renderHelpItem("c", "Copy last response"))
	parts = append(parts, renderHelpItem("Ctrl+Y", "Toggle select mode"))
	parts = append(parts, renderHelpItem("Shift+Mouse", "Select text (native)"))
	parts = append(parts, "")

	// Close hint
	parts = append(parts, dimStyle.Render("Press ? or Esc to close"))

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	helpBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363D")).
		Padding(1, 2).
		Width(min(m.width-4, 50)).
		Render(content)

	return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, helpBox)
}

func renderHelpItem(key, desc string) string {
	return helpKeyStyle.Render(key) + helpDescStyle.Render(desc)
}

// formatTokenCount formats token count for display
func formatTokenCount(count int) string {
	if count >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	}
	if count >= 1000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	return fmt.Sprintf("%d", count)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
