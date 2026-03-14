package widget

import (
	"strings"

	bubbleprogress "github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// ProgressWidget renders a progress bar with an optional label.
type ProgressWidget struct{}

// Init implements Widget.
func (w *ProgressWidget) Init(_ *dom.Node) {}

// Update implements Widget.
func (w *ProgressWidget) Update(_ tea.Msg, _ *dom.Node) UpdateResult {
	return UpdateResult{}
}

// Layout implements Widget.
func (w *ProgressWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// View implements Widget.
func (w *ProgressWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	label := PropString(node, "label", "")
	showPercent := PropBool(node, "show_percent", true)
	styleStr := PropString(node, "style", "")

	labelStyle := lipgloss.NewStyle()
	if ctx.Theme != nil && styleStr != "" {
		labelStyle = ctx.Theme.Resolve(styleStr)
	}

	available := ctx.Width
	var renderedLabel string
	if label != "" {
		renderedLabel = labelStyle.Render(label)
		available -= lipgloss.Width(renderedLabel) + 1
	}
	if available <= 0 {
		return renderedLabel
	}

	opts := []bubbleprogress.Option{bubbleprogress.WithWidth(available)}
	if color := progressColor(styleStr); color != "" {
		opts = append(opts, bubbleprogress.WithSolidFill(color))
	}
	if !showPercent {
		opts = append(opts, bubbleprogress.WithoutPercentage())
	}

	model := bubbleprogress.New(opts...)
	bar := model.ViewAs(progressPercent(node) / 100.0)
	if renderedLabel == "" {
		return bar
	}
	return renderedLabel + " " + bar
}

func progressPercent(node *dom.Node) float64 {
	v, ok := node.GetProp("value")
	if !ok {
		return 0
	}
	var percent float64
	switch val := v.(type) {
	case int:
		percent = float64(val)
	case float64:
		percent = val
	default:
		return 0
	}
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func progressColor(styleStr string) string {
	for _, token := range strings.Fields(styleStr) {
		switch token {
		case "danger":
			return "9"
		case "success":
			return "10"
		case "warning":
			return "11"
		case "info":
			return "12"
		case "muted":
			return "8"
		}
	}
	return ""
}
