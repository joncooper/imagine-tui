package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func tableNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("tbl", dom.TypeTable)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

var testColumns = []any{
	map[string]any{"key": "name", "label": "Name"},
	map[string]any{"key": "status", "label": "Status"},
}

var testRows = []any{
	map[string]any{"name": "Alice", "status": "active"},
	map[string]any{"name": "Bob", "status": "inactive"},
	map[string]any{"name": "Charlie", "status": "active"},
}

func TestTableWidget_View_ShowsHeaders(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{
		"columns": testColumns,
		"rows":    testRows,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "Name") || !strings.Contains(got, "Status") {
		t.Errorf("expected column headers in output, got:\n%s", got)
	}
}

func TestTableWidget_View_ShowsRows(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{
		"columns": testColumns,
		"rows":    testRows,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "Alice") || !strings.Contains(got, "Bob") {
		t.Errorf("expected row data in output, got:\n%s", got)
	}
}

func TestTableWidget_View_Empty(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{
		"columns": testColumns,
		"rows":    []any{},
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Errorf("expected empty placeholder, got:\n%s", got)
	}
}

func TestTableWidget_View_ZeroWidth(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{"columns": testColumns, "rows": testRows})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestTableWidget_Update_ArrowDown(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{"columns": testColumns, "rows": testRows})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if w.selectedRow != 1 {
		t.Errorf("selectedRow = %d, want 1", w.selectedRow)
	}
}

func TestTableWidget_Update_ArrowUp_AtTop(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{"columns": testColumns, "rows": testRows})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyUp}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if w.selectedRow != 0 {
		t.Errorf("selectedRow = %d, want 0 (clamped)", w.selectedRow)
	}
}

func TestTableWidget_Update_ArrowDown_Clamps(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{"columns": testColumns, "rows": testRows})
	w.Init(n)
	w.selectedRow = 2

	w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if w.selectedRow != 2 {
		t.Errorf("selectedRow = %d, want 2 (clamped at last row)", w.selectedRow)
	}
}

func TestTableWidget_Update_Enter_EmitsSelect(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{"columns": testColumns, "rows": testRows})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	found := false
	for _, ev := range result.Events {
		if ev.Type == "select" {
			found = true
			if ev.Data["index"] != 0 {
				t.Errorf("select event index = %v, want 0", ev.Data["index"])
			}
		}
	}
	if !found {
		t.Error("expected select event on Enter")
	}
}

func TestTableWidget_Sort(t *testing.T) {
	w := &TableWidget{}
	sortCols := []any{
		map[string]any{"key": "name", "label": "Name", "sortable": true},
		map[string]any{"key": "status", "label": "Status"},
	}
	n := tableNode(t, map[string]any{
		"columns":  sortCols,
		"rows":     testRows,
		"sortable": true,
	})
	w.Init(n)

	// Sort by name.
	w.sortColumn = "name"
	w.sortAsc = true

	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	// "Alice" should appear before "Bob" and "Charlie" when sorted ascending.
	aliceIdx := strings.Index(got, "Alice")
	bobIdx := strings.Index(got, "Bob")
	if aliceIdx >= bobIdx {
		t.Errorf("expected Alice before Bob in ascending sort")
	}
}

func TestTableWidget_RowStyle_Map(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, map[string]any{
		"columns": testColumns,
		"rows":    testRows,
		"row_style": map[string]any{
			"field": "status",
			"map":   map[string]any{"active": "success", "inactive": "danger"},
		},
	})
	w.Init(n)
	// Should not panic; styling is applied during View.
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "Alice") {
		t.Errorf("expected row content with styling, got:\n%s", got)
	}
}

func TestTableWidget_Expandable(t *testing.T) {
	w := &TableWidget{}
	rows := []any{
		map[string]any{"name": "Alice", "status": "active", "detail": "Alice detail info"},
	}
	n := tableNode(t, map[string]any{
		"columns":    testColumns,
		"rows":       rows,
		"expandable": true,
	})
	w.Init(n)

	// Expand first row.
	w.expandedRows[0] = true
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "Alice detail info") {
		t.Errorf("expected detail in expanded row output, got:\n%s", got)
	}
}

func TestTableWidget_LayoutReturnsNil(t *testing.T) {
	w := &TableWidget{}
	n := tableNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("table Layout should return nil")
	}
}
