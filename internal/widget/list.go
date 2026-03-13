package widget

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// ListWidget implements a vertical item list with selection and badges.
type ListWidget struct {
	selectedIndex int
	filterText    string
	filtered      []int // indices into items
}

type listItem struct {
	ID    string
	Label string
	Badge string
	Style string
}

func (w *ListWidget) Init(node *dom.Node) {
	items := w.parseItems(node)
	w.rebuildFilter(items)

	// Set selected index from prop.
	selID := PropString(node, "selected", "")
	if selID != "" {
		for i, item := range items {
			if item.ID == selID {
				w.selectedIndex = i
				break
			}
		}
	}
}

func (w *ListWidget) parseItems(node *dom.Node) []listItem {
	raw := PropMapSlice(node, "items")
	items := make([]listItem, 0, len(raw))
	for _, m := range raw {
		items = append(items, listItem{
			ID:    stringFromMap(m, "id"),
			Label: stringFromMap(m, "label"),
			Badge: stringFromMap(m, "badge"),
			Style: stringFromMap(m, "style"),
		})
	}
	return items
}

func (w *ListWidget) rebuildFilter(items []listItem) {
	if w.filterText == "" {
		w.filtered = make([]int, len(items))
		for i := range items {
			w.filtered[i] = i
		}
		return
	}
	lower := strings.ToLower(w.filterText)
	w.filtered = nil
	for i, item := range items {
		if strings.Contains(strings.ToLower(item.Label), lower) {
			w.filtered = append(w.filtered, i)
		}
	}
}

func (w *ListWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

func (w *ListWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	items := w.parseItems(node)
	filterable := PropBool(node, "filterable", false)

	switch keyMsg.Type {
	case tea.KeyDown:
		if w.selectedIndex < len(items)-1 {
			w.selectedIndex++
		}
		return UpdateResult{Consumed: true}

	case tea.KeyUp:
		if w.selectedIndex > 0 {
			w.selectedIndex--
		}
		return UpdateResult{Consumed: true}

	case tea.KeyEnter:
		if w.selectedIndex >= 0 && w.selectedIndex < len(items) {
			item := items[w.selectedIndex]
			node.SetProp("selected", item.ID)
			return UpdateResult{
				Consumed: true,
				Events: []WidgetEvent{
					{
						Type:   "select",
						NodeID: node.ID,
						Data:   map[string]any{"id": item.ID, "label": item.Label},
					},
				},
			}
		}
		return UpdateResult{Consumed: true}

	case tea.KeyBackspace:
		if filterable && len(w.filterText) > 0 {
			w.filterText = w.filterText[:len(w.filterText)-1]
			w.rebuildFilter(items)
			w.selectedIndex = 0
			return UpdateResult{Consumed: true}
		}
		return UpdateResult{}

	case tea.KeyRunes:
		if filterable {
			w.filterText += string(keyMsg.Runes)
			w.rebuildFilter(items)
			w.selectedIndex = 0
			return UpdateResult{Consumed: true}
		}
		return UpdateResult{}

	default:
		return UpdateResult{}
	}
}

func (w *ListWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	items := w.parseItems(node)
	if len(items) == 0 {
		style := lipgloss.NewStyle().Width(ctx.Width).Align(lipgloss.Center)
		if ctx.Theme != nil {
			style = mergeStyles(style, ctx.Theme.Resolve("muted"))
		}
		return style.Render("(empty)")
	}

	w.rebuildFilter(items)

	var lines []string
	for _, idx := range w.filtered {
		if idx < 0 || idx >= len(items) {
			continue
		}
		item := items[idx]

		prefix := "  "
		if idx == w.selectedIndex {
			prefix = "▸ "
		}

		label := prefix + item.Label

		// Pad label and right-align badge.
		if item.Badge != "" {
			padLen := ctx.Width - len([]rune(label)) - len([]rune(item.Badge)) - 1
			if padLen < 1 {
				padLen = 1
			}
			label = label + strings.Repeat(" ", padLen) + item.Badge
		}

		style := lipgloss.NewStyle()
		if item.Style != "" && ctx.Theme != nil {
			style = ctx.Theme.Resolve(item.Style)
		}
		if idx == w.selectedIndex {
			style = style.Bold(true)
		}

		lines = append(lines, style.Render(label))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
