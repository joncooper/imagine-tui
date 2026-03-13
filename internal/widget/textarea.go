package widget

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// TextareaWidget implements a multi-line text input.
type TextareaWidget struct {
	value     string
	cursorRow int
	cursorCol int
}

// Init implements Widget.
func (w *TextareaWidget) Init(node *dom.Node) {
	w.value = PropString(node, "value", "")
	lines := strings.Split(w.value, "\n")
	w.cursorRow = len(lines) - 1
	w.cursorCol = len([]rune(lines[w.cursorRow]))
}

// Layout implements Widget.
func (w *TextareaWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// Update implements Widget.
func (w *TextareaWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	oldValue := w.value
	lines := strings.Split(w.value, "\n")

	switch keyMsg.Type {
	case tea.KeyEnter:
		// Insert newline at cursor.
		line := lines[w.cursorRow]
		runes := []rune(line)
		before := string(runes[:w.cursorCol])
		after := string(runes[w.cursorCol:])
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:w.cursorRow]...)
		newLines = append(newLines, before, after)
		if w.cursorRow+1 < len(lines) {
			newLines = append(newLines, lines[w.cursorRow+1:]...)
		}
		w.value = strings.Join(newLines, "\n")
		w.cursorRow++
		w.cursorCol = 0

	case tea.KeyBackspace:
		if w.cursorCol > 0 {
			line := lines[w.cursorRow]
			runes := []rune(line)
			runes = append(runes[:w.cursorCol-1], runes[w.cursorCol:]...)
			lines[w.cursorRow] = string(runes)
			w.cursorCol--
			w.value = strings.Join(lines, "\n")
		} else if w.cursorRow > 0 {
			// Join with previous line.
			prevLine := lines[w.cursorRow-1]
			newCol := len([]rune(prevLine))
			lines[w.cursorRow-1] = prevLine + lines[w.cursorRow]
			lines = append(lines[:w.cursorRow], lines[w.cursorRow+1:]...)
			w.cursorRow--
			w.cursorCol = newCol
			w.value = strings.Join(lines, "\n")
		}

	case tea.KeyUp:
		if w.cursorRow > 0 {
			w.cursorRow--
			lineLen := len([]rune(lines[w.cursorRow]))
			if w.cursorCol > lineLen {
				w.cursorCol = lineLen
			}
		}
		node.SetProp("value", w.value)
		return UpdateResult{Consumed: true}

	case tea.KeyDown:
		if w.cursorRow < len(lines)-1 {
			w.cursorRow++
			lineLen := len([]rune(lines[w.cursorRow]))
			if w.cursorCol > lineLen {
				w.cursorCol = lineLen
			}
		}
		node.SetProp("value", w.value)
		return UpdateResult{Consumed: true}

	case tea.KeyLeft:
		if w.cursorCol > 0 {
			w.cursorCol--
		} else if w.cursorRow > 0 {
			w.cursorRow--
			w.cursorCol = len([]rune(lines[w.cursorRow]))
		}
		return UpdateResult{Consumed: true}

	case tea.KeyRight:
		lineLen := len([]rune(lines[w.cursorRow]))
		if w.cursorCol < lineLen {
			w.cursorCol++
		} else if w.cursorRow < len(lines)-1 {
			w.cursorRow++
			w.cursorCol = 0
		}
		return UpdateResult{Consumed: true}

	case tea.KeyRunes:
		line := lines[w.cursorRow]
		runes := []rune(line)
		for _, r := range keyMsg.Runes {
			runes = append(runes[:w.cursorCol], append([]rune{r}, runes[w.cursorCol:]...)...)
			w.cursorCol++
		}
		lines[w.cursorRow] = string(runes)
		w.value = strings.Join(lines, "\n")

	default:
		return UpdateResult{}
	}

	node.SetProp("value", w.value)

	var events []Event
	if w.value != oldValue {
		events = append(events, Event{
			Type:   "change",
			NodeID: node.ID,
			Data:   map[string]any{"value": w.value},
		})
	}
	return UpdateResult{Consumed: true, Events: events}
}

// View implements Widget.
func (w *TextareaWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	// Sync from prop.
	propVal := PropString(node, "value", "")
	if propVal != w.value {
		w.value = propVal
		lines := strings.Split(w.value, "\n")
		w.cursorRow = len(lines) - 1
		w.cursorCol = len([]rune(lines[w.cursorRow]))
	}

	placeholder := PropString(node, "placeholder", "")
	maxLines := PropInt(node, "max_lines", 0)

	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().
		Border(border).
		Width(ctx.Width - 2)

	if ctx.Focused {
		style = style.BorderForeground(lipgloss.Color("12"))
	}

	content := w.value
	if content == "" {
		content = placeholder
		if ctx.Theme != nil {
			muted := ctx.Theme.Resolve("muted")
			content = muted.Render(content)
		}
	}

	// Apply max_lines as height constraint.
	if maxLines > 0 {
		style = style.MaxHeight(maxLines)
	}

	return style.Render(content)
}
