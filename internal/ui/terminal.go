package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

// Colors and styles
var (
	// Text colors
	UserColor      = color.New(color.FgCyan, color.Bold)
	AssistantColor = color.New(color.FgGreen)
	ErrorColor     = color.New(color.FgRed)
	WarningColor   = color.New(color.FgYellow)
	InfoColor      = color.New(color.FgBlue)
	DimColor       = color.New(color.Faint)

	// Lipgloss styles
	ToolNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginLeft(2)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Bold(true).
			Underline(true)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
)

// Terminal handles terminal I/O and rendering
type Terminal struct {
	reader    *bufio.Reader
	markdown  *MarkdownRenderer
	spinner   *Spinner
	isStreaming bool
}

// NewTerminal creates a new terminal UI
func NewTerminal() *Terminal {
	return &Terminal{
		reader:   bufio.NewReader(os.Stdin),
		markdown: NewMarkdownRenderer(),
		spinner:  NewSpinner(),
	}
}

// PrintWelcome prints the welcome message
func (t *Terminal) PrintWelcome() {
	fmt.Println()
	fmt.Println(HeaderStyle.Render("Claude Code"))
	fmt.Println(DimColor.Sprint("Type your message and press Enter. Use /help for commands."))
	fmt.Println()
}

// PrintPrompt prints the input prompt
func (t *Terminal) PrintPrompt() {
	UserColor.Print("> ")
}

// ReadLine reads a line of input from the user
func (t *Terminal) ReadLine() (string, error) {
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadMultiLine reads multiple lines until a blank line or Ctrl+D
func (t *Terminal) ReadMultiLine() (string, error) {
	var lines []string
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			if len(lines) > 0 {
				return strings.Join(lines, "\n"), nil
			}
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" && len(lines) > 0 {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

// PrintText prints plain text
func (t *Terminal) PrintText(text string) {
	fmt.Print(text)
}

// PrintLine prints a line of text
func (t *Terminal) PrintLine(text string) {
	fmt.Println(text)
}

// PrintAssistantText prints assistant text (streaming)
func (t *Terminal) PrintAssistantText(text string) {
	if !t.isStreaming {
		t.isStreaming = true
		fmt.Println()
		AssistantColor.Print("Claude: ")
	}
	fmt.Print(text)
}

// EndAssistantResponse ends the assistant response
func (t *Terminal) EndAssistantResponse() {
	if t.isStreaming {
		fmt.Println()
		fmt.Println()
		t.isStreaming = false
	}
}

// PrintMarkdown renders and prints markdown
func (t *Terminal) PrintMarkdown(text string) {
	rendered := t.markdown.Render(text)
	fmt.Print(rendered)
}

// PrintToolStart prints the start of a tool execution
func (t *Terminal) PrintToolStart(toolName, toolID string) {
	fmt.Println()
	fmt.Printf("%s %s\n", ToolNameStyle.Render("▶"), ToolNameStyle.Render(toolName))
}

// PrintToolEnd prints the end of a tool execution
func (t *Terminal) PrintToolEnd(toolName string, result string, isError bool) {
	// Truncate long results
	maxLen := 500
	if len(result) > maxLen {
		result = result[:maxLen] + "... (truncated)"
	}

	if isError {
		fmt.Println(ErrorStyle.Render("  ✗ Error: " + result))
	} else {
		// Print result in dimmed style
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Println(ToolResultStyle.Render(line))
			}
		}
	}
	fmt.Println()
}

// PrintError prints an error message
func (t *Terminal) PrintError(err error) {
	fmt.Println()
	ErrorColor.Printf("Error: %s\n", err.Error())
	fmt.Println()
}

// PrintErrorString prints an error string
func (t *Terminal) PrintErrorString(msg string) {
	fmt.Println()
	ErrorColor.Printf("Error: %s\n", msg)
	fmt.Println()
}

// PrintWarning prints a warning message
func (t *Terminal) PrintWarning(msg string) {
	WarningColor.Printf("Warning: %s\n", msg)
}

// PrintInfo prints an info message
func (t *Terminal) PrintInfo(msg string) {
	InfoColor.Println(msg)
}

// PrintSuccess prints a success message
func (t *Terminal) PrintSuccess(msg string) {
	fmt.Println(SuccessStyle.Render("✓ " + msg))
}

// PrintDim prints dimmed text
func (t *Terminal) PrintDim(msg string) {
	DimColor.Println(msg)
}

// PrintBox prints text in a box
func (t *Terminal) PrintBox(title, content string) {
	if title != "" {
		fmt.Println(HeaderStyle.Render(title))
	}
	fmt.Println(BoxStyle.Render(content))
}

// StartSpinner starts the loading spinner
func (t *Terminal) StartSpinner(message string) {
	t.spinner.Start(message)
}

// StopSpinner stops the loading spinner
func (t *Terminal) StopSpinner() {
	t.spinner.Stop()
}

// UpdateSpinner updates the spinner message
func (t *Terminal) UpdateSpinner(message string) {
	t.spinner.UpdateMessage(message)
}

// Clear clears the terminal
func (t *Terminal) Clear() {
	fmt.Print("\033[2J\033[H")
}

// PrintHelp prints help information
func (t *Terminal) PrintHelp() {
	help := `
Commands:
  /help     - Show this help message
  /clear    - Clear the conversation history
  /exit     - Exit the program
  /quit     - Same as /exit

Tips:
  - Type your message and press Enter to send
  - Use Ctrl+C to cancel the current operation
  - Use Ctrl+D to exit
`
	fmt.Println(help)
}
