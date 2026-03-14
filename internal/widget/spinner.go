package widget

import (
	bubblespinner "github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// SpinnerWidget renders an animated spinner with an optional label.
type SpinnerWidget struct {
	model   bubblespinner.Model
	active  bool
	ticking bool
}

// Init implements Widget.
func (w *SpinnerWidget) Init(node *dom.Node) {
	w.model = bubblespinner.New(bubblespinner.WithSpinner(spinnerPreset(node)))
	w.active = PropBool(node, "active", true)
	w.ticking = false
}

// Update implements Widget.
func (w *SpinnerWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	w.sync(node)
	if !w.active {
		w.ticking = false
		return UpdateResult{}
	}

	switch msg.(type) {
	case bubblespinner.TickMsg:
		model, cmd := w.model.Update(msg)
		w.model = model
		w.ticking = cmd != nil
		return UpdateResult{
			Consumed: true,
			Cmd:      cmd,
		}
	default:
		return UpdateResult{}
	}
}

// Layout implements Widget.
func (w *SpinnerWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// View implements Widget.
func (w *SpinnerWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	w.sync(node)

	label := PropString(node, "label", "")
	styleStr := PropString(node, "style", "")
	if ctx.Theme != nil && styleStr != "" {
		w.model.Style = ctx.Theme.Resolve(styleStr)
	} else {
		w.model.Style = lipgloss.NewStyle()
	}

	if !w.active {
		return lipgloss.NewStyle().MaxWidth(ctx.Width).Render(label)
	}

	output := w.model.View()
	if label != "" {
		output = output + " " + label
	}
	return lipgloss.NewStyle().MaxWidth(ctx.Width).Render(output)
}

// Command implements Commander.
func (w *SpinnerWidget) Command(node *dom.Node) tea.Cmd {
	w.sync(node)
	if !w.active {
		w.ticking = false
		return nil
	}
	if w.ticking {
		return nil
	}
	w.ticking = true
	return func() tea.Msg { return w.model.Tick() }
}

func (w *SpinnerWidget) sync(node *dom.Node) {
	w.active = PropBool(node, "active", true)
	w.model.Spinner = spinnerPreset(node)
}

func spinnerPreset(node *dom.Node) bubblespinner.Spinner {
	switch PropString(node, "spinner", "line") {
	case "dot":
		return bubblespinner.Dot
	case "mini_dot":
		return bubblespinner.MiniDot
	case "jump":
		return bubblespinner.Jump
	case "pulse":
		return bubblespinner.Pulse
	case "points":
		return bubblespinner.Points
	case "globe":
		return bubblespinner.Globe
	case "moon":
		return bubblespinner.Moon
	case "monkey":
		return bubblespinner.Monkey
	case "meter":
		return bubblespinner.Meter
	case "hamburger":
		return bubblespinner.Hamburger
	case "ellipsis":
		return bubblespinner.Ellipsis
	default:
		return bubblespinner.Line
	}
}
