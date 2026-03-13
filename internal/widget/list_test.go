package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func listNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("lst", dom.TypeList)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

var testItems = []any{
	map[string]any{"id": "1", "label": "First item"},
	map[string]any{"id": "2", "label": "Second item", "badge": "new"},
	map[string]any{"id": "3", "label": "Third item", "style": "warning"},
}

func TestListWidget_View_ShowsItems(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "First item") || !strings.Contains(got, "Second item") {
		t.Errorf("expected items in output, got:\n%s", got)
	}
}

func TestListWidget_View_ShowsBadges(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "new") {
		t.Errorf("expected badge in output, got:\n%s", got)
	}
}

func TestListWidget_View_Empty(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": []any{}})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Errorf("expected empty placeholder, got:\n%s", got)
	}
}

func TestListWidget_View_ZeroWidth(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestListWidget_Update_ArrowDown(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if w.selectedIndex != 1 {
		t.Errorf("selectedIndex = %d, want 1", w.selectedIndex)
	}
}

func TestListWidget_Update_ArrowUp_AtTop(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems})
	w.Init(n)

	w.Update(tea.KeyMsg{Type: tea.KeyUp}, n)
	if w.selectedIndex != 0 {
		t.Errorf("selectedIndex = %d, want 0", w.selectedIndex)
	}
}

func TestListWidget_Update_Enter_EmitsSelect(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems})
	w.Init(n)
	w.selectedIndex = 1

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	found := false
	for _, ev := range result.Events {
		if ev.Type == "select" && ev.Data["id"] == "2" {
			found = true
		}
	}
	if !found {
		t.Error("expected select event with id=2")
	}
}

func TestListWidget_Filter(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{
		"items":      testItems,
		"filterable": true,
	})
	w.Init(n)

	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, n)
	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, n)
	// "Second item" should match "se".
	if len(w.filtered) != 1 {
		t.Errorf("expected 1 filtered item, got %d", len(w.filtered))
	}
}

func TestListWidget_LayoutReturnsNil(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("list Layout should return nil")
	}
}

func TestListWidget_SelectedCursor(t *testing.T) {
	w := &ListWidget{}
	n := listNode(t, map[string]any{"items": testItems, "selected": "2"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 60, Theme: DefaultTheme()})
	// The selected item should have cursor indicator.
	if !strings.Contains(got, "▸") {
		t.Errorf("expected cursor indicator in output, got:\n%s", got)
	}
}
