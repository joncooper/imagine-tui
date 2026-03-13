package widget

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// TableWidget implements a sortable, scrollable table with expandable rows.
type TableWidget struct {
	selectedRow  int
	sortColumn   string
	sortAsc      bool
	expandedRows map[int]bool
	vp           viewport
}

type tableColumn struct {
	Key      string
	Label    string
	Width    int
	Sortable bool
}

// Init implements Widget.
func (w *TableWidget) Init(_ *dom.Node) {
	w.expandedRows = make(map[int]bool)
}

// Layout implements Widget.
func (w *TableWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

func (w *TableWidget) parseColumns(node *dom.Node) []tableColumn {
	raw := PropMapSlice(node, "columns")
	cols := make([]tableColumn, 0, len(raw))
	for _, m := range raw {
		col := tableColumn{
			Key:   stringFromMap(m, "key"),
			Label: stringFromMap(m, "label"),
		}
		if w, ok := m["width"]; ok {
			switch wi := w.(type) {
			case float64:
				col.Width = int(wi)
			case int:
				col.Width = wi
			}
		}
		if s, ok := m["sortable"]; ok {
			if sb, ok := s.(bool); ok {
				col.Sortable = sb
			}
		}
		cols = append(cols, col)
	}
	return cols
}

// Update implements Widget.
func (w *TableWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	rows := PropMapSlice(node, "rows")
	expandable := PropBool(node, "expandable", false)

	switch keyMsg.Type {
	case tea.KeyDown:
		if w.selectedRow < len(rows)-1 {
			w.selectedRow++
		}
		return UpdateResult{Consumed: true}

	case tea.KeyUp:
		if w.selectedRow > 0 {
			w.selectedRow--
		}
		return UpdateResult{Consumed: true}

	case tea.KeyEnter:
		if expandable {
			w.expandedRows[w.selectedRow] = !w.expandedRows[w.selectedRow]
			return UpdateResult{
				Consumed: true,
				Events: []Event{
					{
						Type:   "expand",
						NodeID: node.ID,
						Data: map[string]any{
							"index":    w.selectedRow,
							"expanded": w.expandedRows[w.selectedRow],
						},
					},
				},
			}
		}
		var rowData map[string]any
		if w.selectedRow >= 0 && w.selectedRow < len(rows) {
			rowData = rows[w.selectedRow]
		}
		return UpdateResult{
			Consumed: true,
			Events: []Event{
				{
					Type:   "select",
					NodeID: node.ID,
					Data:   map[string]any{"index": w.selectedRow, "row": rowData},
				},
			},
		}

	default:
		return UpdateResult{}
	}
}

// View implements Widget.
func (w *TableWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	cols := w.parseColumns(node)
	rows := PropMapSlice(node, "rows")

	if len(rows) == 0 {
		style := lipgloss.NewStyle().Width(ctx.Width).Align(lipgloss.Center)
		if ctx.Theme != nil {
			style = mergeStyles(style, ctx.Theme.Resolve("muted"))
		}
		return style.Render("(empty)")
	}

	// Calculate column widths.
	colWidths := w.calcColumnWidths(cols, rows, ctx.Width)

	// Sort rows if needed.
	sortedRows := w.sortRows(rows, cols)

	// Build row style resolver.
	rowStyler := w.buildRowStyler(node, ctx.Theme)

	// Render header.
	headerCells := make([]string, len(cols))
	headerStyle := lipgloss.NewStyle().Bold(true)
	if ctx.Theme != nil {
		headerStyle = ctx.Theme.Resolve("header")
	}
	for i, col := range cols {
		label := col.Label
		if w.sortColumn == col.Key {
			if w.sortAsc {
				label += " ▲"
			} else {
				label += " ▼"
			}
		}
		headerCells[i] = headerStyle.Width(colWidths[i]).MaxWidth(colWidths[i]).Render(label)
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)

	// Render separator.
	sep := strings.Repeat("─", ctx.Width)

	// Render all rows.
	var allRowLines []string
	for rowIdx, row := range sortedRows {
		rowStyle := lipgloss.NewStyle()
		if rowStyler != nil {
			rowStyle = rowStyler(row)
		}
		if rowIdx == w.selectedRow {
			rowStyle = rowStyle.Background(lipgloss.Color("0")).Foreground(lipgloss.Color("15")).Bold(true)
		}

		cells := make([]string, len(cols))
		for i, col := range cols {
			val := fmt.Sprintf("%v", row[col.Key])
			cells[i] = rowStyle.Width(colWidths[i]).MaxWidth(colWidths[i]).Render(val)
		}
		allRowLines = append(allRowLines, lipgloss.JoinHorizontal(lipgloss.Top, cells...))

		// Expanded detail.
		if w.expandedRows[rowIdx] {
			detail, _ := row["detail"].(string)
			if detail != "" {
				detailStyle := lipgloss.NewStyle().PaddingLeft(2).Width(ctx.Width - 2)
				if ctx.Theme != nil {
					detailStyle = mergeStyles(detailStyle, ctx.Theme.Resolve("muted"))
				}
				allRowLines = append(allRowLines, detailStyle.Render(detail))
			}
		}
	}

	// Viewport scrolling for rows.
	fixedLines := 2 // header + separator
	vpHeight := ctx.Height - fixedLines
	if vpHeight > 0 && len(allRowLines) > vpHeight {
		hintLines := 0
		if vpHeight > 2 {
			hintLines = 2
			vpHeight -= hintLines
		}
		vs := w.vp.slice(len(allRowLines), vpHeight, w.selectedRow)
		visible := allRowLines[vs.Start:vs.End]

		parts := []string{header, sep}
		if vs.Above > 0 {
			hint := scrollHint(vs.Above, true)
			if ctx.Theme != nil {
				hint = ctx.Theme.Resolve("muted").Render(hint)
			}
			parts = append(parts, hint)
		} else if hintLines > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, visible...)
		if vs.Below > 0 {
			hint := scrollHint(vs.Below, false)
			if ctx.Theme != nil {
				hint = ctx.Theme.Resolve("muted").Render(hint)
			}
			parts = append(parts, hint)
		} else if hintLines > 0 {
			parts = append(parts, "")
		}
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	parts := []string{header, sep}
	parts = append(parts, allRowLines...)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (w *TableWidget) calcColumnWidths(cols []tableColumn, _ []map[string]any, totalWidth int) []int {
	n := len(cols)
	if n == 0 {
		return nil
	}

	widths := make([]int, n)
	remaining := totalWidth

	// Assign fixed widths first.
	flexCount := 0
	for i, col := range cols {
		if col.Width > 0 {
			widths[i] = col.Width
			remaining -= col.Width
		} else {
			flexCount++
		}
	}

	// Distribute remaining to flex columns.
	if flexCount > 0 && remaining > 0 {
		each := remaining / flexCount
		for i, col := range cols {
			if col.Width == 0 {
				widths[i] = each
			}
		}
	}

	return widths
}

func (w *TableWidget) sortRows(rows []map[string]any, _ []tableColumn) []map[string]any {
	if w.sortColumn == "" {
		return rows
	}

	sorted := make([]map[string]any, len(rows))
	copy(sorted, rows)

	sort.SliceStable(sorted, func(i, j int) bool {
		vi := fmt.Sprintf("%v", sorted[i][w.sortColumn])
		vj := fmt.Sprintf("%v", sorted[j][w.sortColumn])
		if w.sortAsc {
			return vi < vj
		}
		return vi > vj
	})

	return sorted
}

type rowStyleFunc func(row map[string]any) lipgloss.Style

func (w *TableWidget) buildRowStyler(node *dom.Node, theme *Theme) rowStyleFunc {
	rs, ok := node.GetProp("row_style")
	if !ok || theme == nil {
		return nil
	}

	// Map mode: {"field": "status", "map": {"active": "success", ...}}
	rsMap, ok := rs.(map[string]any)
	if !ok {
		return nil
	}

	field, _ := rsMap["field"].(string)
	styleMap, _ := rsMap["map"].(map[string]any)
	if field == "" || styleMap == nil {
		return nil
	}

	return func(row map[string]any) lipgloss.Style {
		val := fmt.Sprintf("%v", row[field])
		if token, ok := styleMap[val].(string); ok {
			return theme.Resolve(token)
		}
		return lipgloss.NewStyle()
	}
}
