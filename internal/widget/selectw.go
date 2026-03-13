package widget

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// SelectWidget implements single/multi-select from an options list.
type SelectWidget struct {
	open        bool
	highlighted int
	filterText  string
	filtered    []int // indices into options that match filter
	selected    map[string]bool
	multi       bool
}

type selectOption struct {
	Label string
	Value string
}

// Init implements Widget.
func (w *SelectWidget) Init(node *dom.Node) {
	w.multi = PropBool(node, "multi", false)
	w.selected = make(map[string]bool)

	// Parse selected value(s).
	sel, _ := node.GetProp("selected")
	switch v := sel.(type) {
	case string:
		if v != "" {
			w.selected[v] = true
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				w.selected[s] = true
			}
		}
	}

	// Set highlighted to match first selected.
	opts := w.parseOptions(node)
	for i, opt := range opts {
		if w.selected[opt.Value] {
			w.highlighted = i
			break
		}
	}

	w.rebuildFilter(opts)
}

func (w *SelectWidget) parseOptions(node *dom.Node) []selectOption {
	raw := PropMapSlice(node, "options")
	opts := make([]selectOption, 0, len(raw))
	for _, m := range raw {
		label, _ := m["label"].(string)
		value, _ := m["value"].(string)
		opts = append(opts, selectOption{Label: label, Value: value})
	}
	return opts
}

func (w *SelectWidget) rebuildFilter(opts []selectOption) {
	if w.filterText == "" {
		w.filtered = make([]int, len(opts))
		for i := range opts {
			w.filtered[i] = i
		}
		return
	}
	lower := strings.ToLower(w.filterText)
	w.filtered = nil
	for i, opt := range opts {
		if strings.Contains(strings.ToLower(opt.Label), lower) {
			w.filtered = append(w.filtered, i)
		}
	}
}

// Layout implements Widget.
func (w *SelectWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// Update implements Widget.
func (w *SelectWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	opts := w.parseOptions(node)
	filterable := PropBool(node, "filterable", false)

	switch keyMsg.Type {
	case tea.KeyEnter:
		if !w.open {
			w.open = true
			return UpdateResult{Consumed: true}
		}
		return w.selectCurrent(opts, node)

	case tea.KeyEscape:
		if w.open {
			w.open = false
			w.filterText = ""
			w.rebuildFilter(opts)
			return UpdateResult{Consumed: true}
		}
		return UpdateResult{}

	case tea.KeyDown:
		if !w.open {
			return UpdateResult{}
		}
		if len(w.filtered) > 0 {
			// Find current position in filtered list.
			filterIdx := w.filterIndex()
			filterIdx++
			if filterIdx >= len(w.filtered) {
				filterIdx = 0
			}
			w.highlighted = w.filtered[filterIdx]
		}
		return UpdateResult{Consumed: true}

	case tea.KeyUp:
		if !w.open {
			return UpdateResult{}
		}
		if len(w.filtered) > 0 {
			filterIdx := w.filterIndex()
			filterIdx--
			if filterIdx < 0 {
				filterIdx = len(w.filtered) - 1
			}
			w.highlighted = w.filtered[filterIdx]
		}
		return UpdateResult{Consumed: true}

	case tea.KeyBackspace:
		if w.open && filterable && w.filterText != "" {
			w.filterText = w.filterText[:len(w.filterText)-1]
			w.rebuildFilter(opts)
			w.highlighted = 0
			if len(w.filtered) > 0 {
				w.highlighted = w.filtered[0]
			}
			return UpdateResult{Consumed: true}
		}
		return UpdateResult{}

	case tea.KeyRunes:
		if w.open && filterable {
			w.filterText += string(keyMsg.Runes)
			w.rebuildFilter(opts)
			w.highlighted = 0
			if len(w.filtered) > 0 {
				w.highlighted = w.filtered[0]
			}
			return UpdateResult{Consumed: true}
		}
		return UpdateResult{}

	default:
		return UpdateResult{}
	}
}

func (w *SelectWidget) filterIndex() int {
	for i, idx := range w.filtered {
		if idx == w.highlighted {
			return i
		}
	}
	return 0
}

func (w *SelectWidget) selectCurrent(opts []selectOption, node *dom.Node) UpdateResult {
	if w.highlighted < 0 || w.highlighted >= len(opts) {
		return UpdateResult{Consumed: true}
	}

	opt := opts[w.highlighted]

	if w.multi {
		if w.selected[opt.Value] {
			delete(w.selected, opt.Value)
		} else {
			w.selected[opt.Value] = true
		}
		// Build selected slice preserving option order.
		var selected []string
		for _, o := range opts {
			if w.selected[o.Value] {
				selected = append(selected, o.Value)
			}
		}
		node.SetProp("selected", selected)
		return UpdateResult{
			Consumed: true,
			Events: []Event{
				{Type: "change", NodeID: node.ID, Data: map[string]any{"selected": selected}},
			},
		}
	}

	// Single select.
	w.selected = map[string]bool{opt.Value: true}
	w.open = false
	w.filterText = ""
	w.rebuildFilter(opts)
	node.SetProp("selected", opt.Value)
	return UpdateResult{
		Consumed: true,
		Events: []Event{
			{Type: "change", NodeID: node.ID, Data: map[string]any{"selected": opt.Value}},
		},
	}
}

// View implements Widget.
func (w *SelectWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	opts := w.parseOptions(node)
	styleStr := PropString(node, "style", "")

	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().
		Border(border).
		Width(ctx.Width - 2)

	if ctx.Focused {
		style = style.BorderForeground(lipgloss.Color("12"))
	}

	if ctx.Theme != nil && styleStr != "" {
		tokenStyle := ctx.Theme.Resolve(styleStr)
		style = mergeStyles(style, tokenStyle)
	}

	if !w.open {
		// Show selected label.
		label := w.selectedLabels(opts)
		if label == "" {
			label = "(none)"
			if ctx.Theme != nil {
				label = ctx.Theme.Resolve("muted").Render(label)
			}
		}
		return style.Render(label + " ▼")
	}

	// Open: show option list.
	var lines []string
	for _, idx := range w.filtered {
		if idx < 0 || idx >= len(opts) {
			continue
		}
		opt := opts[idx]
		prefix := "  "
		if w.selected[opt.Value] {
			prefix = "✓ "
		}
		line := prefix + opt.Label
		if idx == w.highlighted {
			if ctx.Theme != nil {
				line = ctx.Theme.Resolve("selected").Render(line)
			}
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	if w.filterText != "" {
		content = "Filter: " + w.filterText + "\n" + content
	}

	return style.Render(content)
}

func (w *SelectWidget) selectedLabels(opts []selectOption) string {
	var labels []string
	for _, opt := range opts {
		if w.selected[opt.Value] {
			labels = append(labels, opt.Label)
		}
	}
	return strings.Join(labels, ", ")
}
