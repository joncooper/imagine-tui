package widget

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// ButtonWidget implements a focusable action trigger.
type ButtonWidget struct{}

func (w *ButtonWidget) Init(_ *dom.Node) {}

func (w *ButtonWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

func (w *ButtonWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	disabled := PropBool(node, "disabled", false)

	switch keyMsg.Type {
	case tea.KeyEnter, tea.KeySpace:
		if disabled {
			return UpdateResult{Consumed: true}
		}
		return UpdateResult{
			Consumed: true,
			Events: []WidgetEvent{
				{Type: "click", NodeID: node.ID, Data: map[string]any{}},
			},
		}
	default:
		return UpdateResult{}
	}
}

func (w *ButtonWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	label := PropString(node, "label", "")
	disabled := PropBool(node, "disabled", false)
	styleStr := PropString(node, "style", "")

	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().
		Border(border).
		Padding(0, 1).
		Align(lipgloss.Center)

	if disabled {
		style = style.Faint(true).Foreground(lipgloss.Color("8"))
	} else if ctx.Focused {
		style = style.BorderForeground(lipgloss.Color("12")).Bold(true)
	}

	if ctx.Theme != nil && styleStr != "" {
		tokenStyle := ctx.Theme.Resolve(styleStr)
		style = mergeStyles(style, tokenStyle)
	}

	return style.Render(label)
}
