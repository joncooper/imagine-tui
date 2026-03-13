package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func textareaNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("ta", dom.TypeTextarea)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestTextareaWidget_Init(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": "line1\nline2"})
	w.Init(n)
	if w.value != "line1\nline2" {
		t.Errorf("value = %q, want %q", w.value, "line1\nline2")
	}
}

func TestTextareaWidget_View_WithValue(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": "hello\nworld"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Height: 10, Theme: DefaultTheme()})
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Errorf("expected both lines in output, got %q", got)
	}
}

func TestTextareaWidget_View_Placeholder(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"placeholder": "enter text..."})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Height: 10, Theme: DefaultTheme()})
	if !strings.Contains(got, "enter text...") {
		t.Errorf("expected placeholder in output, got %q", got)
	}
}

func TestTextareaWidget_View_ZeroWidth(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": "test"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestTextareaWidget_Update_TypeCharacter(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": ""})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if w.value != "a" {
		t.Errorf("value = %q, want %q", w.value, "a")
	}
}

func TestTextareaWidget_Update_Enter_InsertsNewline(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": "line1"})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if !strings.Contains(w.value, "\n") {
		t.Errorf("expected newline in value, got %q", w.value)
	}
}

func TestTextareaWidget_Update_Backspace(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": "abc"})
	w.Init(n)

	w.Update(tea.KeyMsg{Type: tea.KeyBackspace}, n)
	if w.value != "ab" {
		t.Errorf("value = %q, want %q", w.value, "ab")
	}
}

func TestTextareaWidget_Update_ChangeEvent(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{"value": ""})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, n)
	found := false
	for _, ev := range result.Events {
		if ev.Type == "change" {
			found = true
		}
	}
	if !found {
		t.Error("expected change event")
	}
}

func TestTextareaWidget_View_MaxLines(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, map[string]any{
		"value":     "1\n2\n3\n4\n5\n6\n7\n8\n9\n10",
		"max_lines": 3,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Height: 0, Theme: DefaultTheme()})
	// The rendered output should respect max_lines for visible area.
	_ = got // just verify no panic
}

func TestTextareaWidget_LayoutReturnsNil(t *testing.T) {
	w := &TextareaWidget{}
	n := textareaNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("textarea Layout should return nil")
	}
}

func TestTextareaWidget_ScrollDown(t *testing.T) {
	w := &TextareaWidget{}
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	n := textareaNode(t, map[string]any{
		"value":     strings.Join(lines, "\n"),
		"max_lines": 5,
	})
	w.Init(n)

	// Scroll down.
	w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	// Cursor should have moved.
	if w.cursorRow < 2 {
		t.Errorf("cursorRow = %d, expected >= 2 after two down presses", w.cursorRow)
	}
}
