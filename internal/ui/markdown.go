package ui

import (
	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer renders markdown to terminal output
type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
}

// NewMarkdownRenderer creates a new markdown renderer
func NewMarkdownRenderer() *MarkdownRenderer {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// Fallback to a basic renderer if auto style fails
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(100),
		)
	}

	return &MarkdownRenderer{
		renderer: renderer,
	}
}

// Render renders markdown to terminal-formatted text
func (m *MarkdownRenderer) Render(text string) string {
	if m.renderer == nil {
		return text
	}

	rendered, err := m.renderer.Render(text)
	if err != nil {
		return text
	}

	return rendered
}

// RenderCodeBlock renders a code block with optional language
func (m *MarkdownRenderer) RenderCodeBlock(code, language string) string {
	if language != "" {
		code = "```" + language + "\n" + code + "\n```"
	} else {
		code = "```\n" + code + "\n```"
	}
	return m.Render(code)
}
