package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func selectNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("sel", dom.TypeSelect)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

var testOptions = []any{
	map[string]any{"label": "Alpha", "value": "a"},
	map[string]any{"label": "Beta", "value": "b"},
	map[string]any{"label": "Charlie", "value": "c"},
}

func TestSelectWidget_Init(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{
		"options":  testOptions,
		"selected": "b",
	})
	w.Init(n)
	if w.highlighted != 1 {
		t.Errorf("highlighted = %d, want 1 (matching 'b')", w.highlighted)
	}
}

func TestSelectWidget_View_ShowsSelected(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{
		"options":  testOptions,
		"selected": "a",
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "Alpha") {
		t.Errorf("expected selected label in output, got %q", got)
	}
}

func TestSelectWidget_View_ZeroWidth(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestSelectWidget_Update_ArrowDown(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)
	w.open = true

	w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if w.highlighted != 1 {
		t.Errorf("highlighted = %d, want 1 after down", w.highlighted)
	}
}

func TestSelectWidget_Update_ArrowUp(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)
	w.open = true
	w.highlighted = 2

	w.Update(tea.KeyMsg{Type: tea.KeyUp}, n)
	if w.highlighted != 1 {
		t.Errorf("highlighted = %d, want 1 after up", w.highlighted)
	}
}

func TestSelectWidget_Update_ArrowDown_Wraps(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)
	w.open = true
	w.highlighted = 2

	w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if w.highlighted != 0 {
		t.Errorf("highlighted = %d, want 0 (wrap)", w.highlighted)
	}
}

func TestSelectWidget_Update_Enter_Selects(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)
	w.open = true
	w.highlighted = 1

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}

	// Should have emitted a change event.
	found := false
	for _, ev := range result.Events {
		if ev.Type == "change" && ev.Data["selected"] == "b" {
			found = true
		}
	}
	if !found {
		t.Error("expected change event with selected=b")
	}

	// Value should be written back to prop.
	v, _ := n.GetProp("selected")
	if v != "b" {
		t.Errorf("selected prop = %v, want %q", v, "b")
	}
}

func TestSelectWidget_Update_Enter_OpensWhenClosed(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)

	w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	if !w.open {
		t.Error("expected select to open on Enter")
	}
}

func TestSelectWidget_Update_Escape_Closes(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{"options": testOptions})
	w.Init(n)
	w.open = true

	w.Update(tea.KeyMsg{Type: tea.KeyEscape}, n)
	if w.open {
		t.Error("expected select to close on Escape")
	}
}

func TestSelectWidget_MultiSelect(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{
		"options":  testOptions,
		"multi":    true,
		"selected": []any{"a"},
	})
	w.Init(n)
	w.open = true
	w.highlighted = 1

	// Toggle "b".
	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	found := false
	for _, ev := range result.Events {
		if ev.Type == "change" {
			sel, ok := ev.Data["selected"].([]string)
			if ok && len(sel) == 2 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected change event with 2 selections after multi-toggle")
	}
}

func TestSelectWidget_Filter(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, map[string]any{
		"options":    testOptions,
		"filterable": true,
	})
	w.Init(n)
	w.open = true

	// Type "ch" to filter.
	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}, n)
	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, n)
	if len(w.filtered) != 1 {
		t.Errorf("expected 1 filtered option (Charlie), got %d", len(w.filtered))
	}
}

func TestSelectWidget_LayoutReturnsNil(t *testing.T) {
	w := &SelectWidget{}
	n := selectNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("select Layout should return nil")
	}
}
