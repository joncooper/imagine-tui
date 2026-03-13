package widget

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// LogWidget implements an append-only scrolling log with severity coloring.
type LogWidget struct {
	scrollOffset int
	stickyBottom bool
}

type logLine struct {
	Text      string
	Level     string
	Timestamp string
}

func (w *LogWidget) Init(node *dom.Node) {
	w.stickyBottom = PropBool(node, "auto_scroll", true)
}

func (w *LogWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

func (w *LogWidget) parseLines(node *dom.Node) []logLine {
	raw := PropMapSlice(node, "lines")
	lines := make([]logLine, 0, len(raw))
	for _, m := range raw {
		lines = append(lines, logLine{
			Text:      stringFromMap(m, "text"),
			Level:     stringFromMap(m, "level"),
			Timestamp: stringFromMap(m, "timestamp"),
		})
	}
	return lines
}

func (w *LogWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	lines := w.parseLines(node)

	switch keyMsg.Type {
	case tea.KeyDown:
		w.scrollOffset++
		if w.scrollOffset >= len(lines) {
			w.scrollOffset = len(lines) - 1
		}
		// Re-engage sticky if at bottom.
		if w.scrollOffset >= len(lines)-1 {
			w.stickyBottom = true
		}
		return UpdateResult{Consumed: true}

	case tea.KeyUp:
		if w.scrollOffset > 0 {
			w.scrollOffset--
			w.stickyBottom = false
		}
		return UpdateResult{Consumed: true}

	default:
		return UpdateResult{}
	}
}

func (w *LogWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	lines := w.parseLines(node)
	maxLines := PropInt(node, "max_lines", 1000)

	if len(lines) == 0 {
		style := lipgloss.NewStyle().Width(ctx.Width).Align(lipgloss.Center)
		if ctx.Theme != nil {
			style = mergeStyles(style, ctx.Theme.Resolve("muted"))
		}
		return style.Render("(empty)")
	}

	// Truncate oldest if exceeding max.
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	// Sticky-bottom: show last N lines.
	if w.stickyBottom {
		w.scrollOffset = len(lines) // will be clamped by viewport
	}

	var rendered []string
	for _, line := range lines {
		rendered = append(rendered, w.renderLogLine(line, ctx))
	}

	return strings.Join(rendered, "\n")
}

func (w *LogWidget) renderLogLine(line logLine, ctx ViewContext) string {
	var parts []string

	// Timestamp.
	if line.Timestamp != "" {
		ts := line.Timestamp
		if ctx.Theme != nil {
			ts = ctx.Theme.Resolve("muted").Render(ts)
		}
		parts = append(parts, ts)
	}

	// Level badge.
	levelStr := w.formatLevel(line.Level, ctx.Theme)
	if levelStr != "" {
		parts = append(parts, levelStr)
	}

	// Content — rendered verbatim (ANSI passthrough).
	parts = append(parts, line.Text)

	return strings.Join(parts, " ")
}

func (w *LogWidget) formatLevel(level string, theme *Theme) string {
	if level == "" {
		return ""
	}

	badge := "[" + strings.ToUpper(level) + "]"
	if theme == nil {
		return badge
	}

	var token string
	switch level {
	case "error":
		token = "danger"
	case "warn":
		token = "warning"
	case "debug":
		token = "muted"
	default:
		return badge
	}

	return theme.Resolve(token).Render(badge)
}
