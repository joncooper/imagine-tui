package widget

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// TextWidget renders styled text content.
type TextWidget struct{}

// Init implements Widget.
func (w *TextWidget) Init(_ *dom.Node) {}

// Update implements Widget.
func (w *TextWidget) Update(_ tea.Msg, _ *dom.Node) UpdateResult {
	return UpdateResult{}
}

// Layout implements Widget.
func (w *TextWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// View implements Widget.
func (w *TextWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	segments := PropMapSlice(node, "segments")
	if len(segments) > 0 {
		return w.renderSegments(segments, ctx)
	}

	text := PropString(node, "content", "")
	if text == "" {
		text = PropString(node, "text", "")
	}
	styleStr := PropString(node, "style", "")
	wrap := PropBool(node, "wrap", true)
	maxLines := PropInt(node, "max_lines", 0)

	style := lipgloss.NewStyle()
	if ctx.Theme != nil && styleStr != "" {
		style = ctx.Theme.Resolve(styleStr)
	}

	if wrap {
		style = style.Width(ctx.Width)
	}

	rendered := style.Render(text)

	if maxLines > 0 {
		rendered = truncateLines(rendered, maxLines)
	}

	return rendered
}

func (w *TextWidget) renderSegments(segments []map[string]any, ctx ViewContext) string {
	var parts []string
	for _, seg := range segments {
		text, _ := seg["text"].(string)
		styleStr, _ := seg["style"].(string)

		style := lipgloss.NewStyle()
		if ctx.Theme != nil && styleStr != "" {
			style = ctx.Theme.Resolve(styleStr)
		}
		parts = append(parts, style.Render(text))
	}
	return strings.Join(parts, "")
}

func truncateLines(s string, limit int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= limit {
		return s
	}
	return strings.Join(lines[:limit], "\n")
}
