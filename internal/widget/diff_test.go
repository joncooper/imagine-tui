package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func diffNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("diff", dom.TypeDiff)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

var testHunks = []any{
	map[string]any{
		"old_start": 10,
		"new_start": 10,
		"lines": []any{
			map[string]any{"type": "context", "content": "func main() {", "old_num": 10, "new_num": 10},
			map[string]any{"type": "remove", "content": "    fmt.Println(\"old\")", "old_num": 11},
			map[string]any{"type": "add", "content": "    fmt.Println(\"new\")", "new_num": 11},
			map[string]any{"type": "context", "content": "}", "old_num": 12, "new_num": 12},
		},
	},
}

func TestDiffWidget_View_Unified(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{
		"hunks":     testHunks,
		"file_name": "main.go",
		"mode":      "unified",
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected file name in output, got:\n%s", got)
	}
	if !strings.Contains(got, "func main()") {
		t.Errorf("expected context line in output, got:\n%s", got)
	}
}

func TestDiffWidget_View_ShowsAddRemove(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{"hunks": testHunks, "mode": "unified"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "old") || !strings.Contains(got, "new") {
		t.Errorf("expected add/remove content, got:\n%s", got)
	}
}

func TestDiffWidget_View_ZeroWidth(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{"hunks": testHunks})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestDiffWidget_View_EmptyHunks(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{"hunks": []any{}})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Errorf("expected empty placeholder, got:\n%s", got)
	}
}

func TestDiffWidget_Update_ToggleMode(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{"hunks": testHunks, "mode": "unified"})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, n)
	if !result.Consumed {
		t.Error("expected 'd' consumed")
	}
	if w.mode != "split" {
		t.Errorf("mode = %q, want %q after toggle", w.mode, "split")
	}

	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, n)
	if w.mode != "unified" {
		t.Errorf("mode = %q, want %q after second toggle", w.mode, "unified")
	}
}

func TestDiffWidget_Update_NextPrevHunk(t *testing.T) {
	twoHunks := []any{
		map[string]any{"old_start": 1, "new_start": 1, "lines": []any{
			map[string]any{"type": "context", "content": "a", "old_num": 1, "new_num": 1},
		}},
		map[string]any{"old_start": 10, "new_start": 10, "lines": []any{
			map[string]any{"type": "context", "content": "b", "old_num": 10, "new_num": 10},
		}},
	}
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{"hunks": twoHunks})
	w.Init(n)

	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, n)
	if w.currentHunk != 1 {
		t.Errorf("currentHunk = %d, want 1 after 'n'", w.currentHunk)
	}

	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}, n)
	if w.currentHunk != 0 {
		t.Errorf("currentHunk = %d, want 0 after 'p'", w.currentHunk)
	}
}

func TestDiffWidget_Update_SelectLine(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, map[string]any{"hunks": testHunks})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	found := false
	for _, ev := range result.Events {
		if ev.Type == "select_line" {
			found = true
		}
	}
	if !found {
		t.Error("expected select_line event on Enter")
	}
}

func TestDiffWidget_LayoutReturnsNil(t *testing.T) {
	w := &DiffWidget{}
	n := diffNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("diff Layout should return nil")
	}
}
