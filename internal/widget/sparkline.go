package widget

import (
	"math"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

var sparklineLevels = []rune("▁▂▃▄▅▆▇█")

// SparklineWidget renders a compact numeric trend line.
type SparklineWidget struct{}

// Init implements Widget.
func (w *SparklineWidget) Init(_ *dom.Node) {}

// Update implements Widget.
func (w *SparklineWidget) Update(_ tea.Msg, _ *dom.Node) UpdateResult {
	return UpdateResult{}
}

// Layout implements Widget.
func (w *SparklineWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// View implements Widget.
func (w *SparklineWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	values := sparklineValues(node)
	if len(values) == 0 {
		return "(empty)"
	}

	label := PropString(node, "label", "")
	available := ctx.Width
	if label != "" {
		available -= lipgloss.Width(label) + 1
	}
	if available <= 0 {
		return label
	}

	if len(values) > available {
		values = values[len(values)-available:]
	}

	lower, upper := sparklineBounds(node, values)
	runes := make([]rune, len(values))
	for i, value := range values {
		runes[i] = sparklineRune(value, lower, upper)
	}

	result := string(runes)
	if label != "" {
		result = label + " " + result
	}

	styleStr := PropString(node, "style", "")
	if ctx.Theme != nil && styleStr != "" {
		result = ctx.Theme.Resolve(styleStr).Render(result)
	}
	return result
}

func sparklineValues(node *dom.Node) []float64 {
	raw, ok := node.GetProp("values")
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	values := make([]float64, 0, len(items))
	for _, item := range items {
		switch val := item.(type) {
		case int:
			values = append(values, float64(val))
		case float64:
			values = append(values, val)
		}
	}
	return values
}

func sparklineBounds(node *dom.Node, values []float64) (lower, upper float64) {
	lower, minOK := numericProp(node, "min")
	upper, maxOK := numericProp(node, "max")
	if !minOK || !maxOK {
		derivedMin, derivedMax := values[0], values[0]
		for _, value := range values[1:] {
			derivedMin = math.Min(derivedMin, value)
			derivedMax = math.Max(derivedMax, value)
		}
		if !minOK {
			lower = derivedMin
		}
		if !maxOK {
			upper = derivedMax
		}
	}
	if upper < lower {
		return upper, lower
	}
	return lower, upper
}

func sparklineRune(value, lower, upper float64) rune {
	if upper == lower {
		return sparklineLevels[len(sparklineLevels)-1]
	}
	ratio := (value - lower) / (upper - lower)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	idx := int(math.Round(ratio * float64(len(sparklineLevels)-1)))
	return sparklineLevels[idx]
}

func numericProp(node *dom.Node, key string) (float64, bool) {
	v, ok := node.GetProp(key)
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return float64(val), true
	case float64:
		return val, true
	default:
		return 0, false
	}
}
