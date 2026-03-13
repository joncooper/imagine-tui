package widget

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// InputWidget implements a single-line text input.
type InputWidget struct {
	value     string
	cursorPos int
	pattern   *regexp.Regexp
	valid     bool
}

func (w *InputWidget) Init(node *dom.Node) {
	w.value = PropString(node, "value", "")
	w.cursorPos = len([]rune(w.value))
	w.compilePattern(node)
	w.validate()
}

func (w *InputWidget) compilePattern(node *dom.Node) {
	patStr := PropString(node, "pattern", "")
	if patStr == "" {
		w.pattern = nil
		return
	}
	re, err := regexp.Compile(patStr)
	if err != nil {
		w.pattern = nil // bad regex → no validation
		return
	}
	w.pattern = re
}

func (w *InputWidget) validate() {
	if w.pattern == nil {
		w.valid = true
		return
	}
	w.valid = w.pattern.MatchString(w.value)
}

func (w *InputWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

func (w *InputWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	oldValue := w.value
	runes := []rune(w.value)

	switch keyMsg.Type {
	case tea.KeyEnter:
		return UpdateResult{
			Consumed: true,
			Events: []WidgetEvent{
				{Type: "submit", NodeID: node.ID, Data: map[string]any{"value": w.value}},
			},
		}

	case tea.KeyBackspace:
		if w.cursorPos > 0 {
			runes = append(runes[:w.cursorPos-1], runes[w.cursorPos:]...)
			w.cursorPos--
			w.value = string(runes)
		}

	case tea.KeyDelete:
		if w.cursorPos < len(runes) {
			runes = append(runes[:w.cursorPos], runes[w.cursorPos+1:]...)
			w.value = string(runes)
		}

	case tea.KeyLeft:
		if w.cursorPos > 0 {
			w.cursorPos--
		}
		return UpdateResult{Consumed: true}

	case tea.KeyRight:
		if w.cursorPos < len(runes) {
			w.cursorPos++
		}
		return UpdateResult{Consumed: true}

	case tea.KeyHome:
		w.cursorPos = 0
		return UpdateResult{Consumed: true}

	case tea.KeyEnd:
		w.cursorPos = len(runes)
		return UpdateResult{Consumed: true}

	case tea.KeyRunes:
		for _, r := range keyMsg.Runes {
			runes = append(runes[:w.cursorPos], append([]rune{r}, runes[w.cursorPos:]...)...)
			w.cursorPos++
		}
		w.value = string(runes)

	default:
		return UpdateResult{}
	}

	// Write back value to DOM.
	node.SetProp("value", w.value)
	w.compilePattern(node)
	w.validate()

	var events []WidgetEvent
	if w.value != oldValue {
		events = append(events, WidgetEvent{
			Type:   "change",
			NodeID: node.ID,
			Data:   map[string]any{"value": w.value},
		})
	}

	return UpdateResult{Consumed: true, Events: events}
}

func (w *InputWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	// Sync value from node prop (may have been changed externally).
	propVal := PropString(node, "value", "")
	if propVal != w.value {
		w.value = propVal
		w.cursorPos = len([]rune(w.value))
		w.compilePattern(node)
		w.validate()
	}

	placeholder := PropString(node, "placeholder", "")
	styleStr := PropString(node, "style", "")

	// Base style with border.
	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().
		Border(border).
		Width(ctx.Width - 2) // account for border

	// Validation border color.
	if w.pattern != nil {
		if w.valid {
			style = style.BorderForeground(lipgloss.Color("10")) // green
		} else {
			style = style.BorderForeground(lipgloss.Color("9")) // red
		}
	}
	if ctx.Focused {
		style = style.BorderForeground(lipgloss.Color("12")) // blue
	}

	// Apply style tokens.
	if ctx.Theme != nil && styleStr != "" {
		tokenStyle := ctx.Theme.Resolve(styleStr)
		style = mergeStyles(style, tokenStyle)
	}

	// Content.
	content := w.value
	if content == "" && !ctx.Focused {
		content = placeholder
		// Render placeholder in muted style.
		if ctx.Theme != nil {
			muted := ctx.Theme.Resolve("muted")
			content = muted.Render(content)
		}
	}

	// Show cursor when focused.
	if ctx.Focused && content == "" {
		content = placeholder
	}

	return style.Render(strings.TrimRight(content, "\n"))
}
