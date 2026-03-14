package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func inputNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("inp", dom.TypeInput)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestInputWidget_Init(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "hello"})
	w.Init(n)
	if w.value != "hello" {
		t.Errorf("value = %q, want %q", w.value, "hello")
	}
}

func TestInputWidget_View_Placeholder(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"placeholder": "type here..."})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "type here...") {
		t.Errorf("expected placeholder in output, got %q", got)
	}
}

func TestInputWidget_View_WithValue(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "hello"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "hello") {
		t.Errorf("expected value in output, got %q", got)
	}
}

func TestInputWidget_View_ZeroWidth(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "hello"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestInputWidget_Update_KeyPress(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "hel"})
	w.Init(n)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	result := w.Update(msg, n)
	if !result.Consumed {
		t.Error("expected keypress to be consumed")
	}
	if w.value != "hell" {
		t.Errorf("value = %q, want %q", w.value, "hell")
	}
	// Should write back to node.
	v, _ := n.GetProp("value")
	if v != "hell" {
		t.Errorf("node prop value = %v, want %q", v, "hell")
	}
}

func TestInputWidget_Update_Backspace(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "hello"})
	w.Init(n)

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	result := w.Update(msg, n)
	if !result.Consumed {
		t.Error("expected backspace to be consumed")
	}
	if w.value != "hell" {
		t.Errorf("value = %q, want %q", w.value, "hell")
	}
}

func TestInputWidget_Update_Enter_EmitsSubmit(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "done"})
	w.Init(n)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result := w.Update(msg, n)
	if !result.Consumed {
		t.Error("expected Enter to be consumed")
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].Type != "submit" {
		t.Errorf("event type = %q, want %q", result.Events[0].Type, "submit")
	}
	if result.Events[0].Data["value"] != "done" {
		t.Errorf("event data value = %v, want %q", result.Events[0].Data["value"], "done")
	}
}

func TestInputWidget_Update_ChangeEvent(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": ""})
	w.Init(n)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	result := w.Update(msg, n)
	// Should emit a change event.
	found := false
	for _, ev := range result.Events {
		if ev.Type == "change" {
			found = true
			if ev.Data["value"] != "a" {
				t.Errorf("change event value = %v, want %q", ev.Data["value"], "a")
			}
		}
	}
	if !found {
		t.Error("expected change event on keystroke")
	}
}

func TestInputWidget_Update_SpaceKey(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "survey"})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeySpace}, n)
	if !result.Consumed {
		t.Error("expected space key to be consumed")
	}
	if w.value != "survey " {
		t.Errorf("value = %q, want %q", w.value, "survey ")
	}
	if got, _ := n.GetProp("value"); got != "survey " {
		t.Errorf("node prop value = %v, want %q", got, "survey ")
	}
	if len(result.Events) != 1 || result.Events[0].Type != "change" {
		t.Fatalf("expected one change event, got %#v", result.Events)
	}
	if result.Events[0].Data["value"] != "survey " {
		t.Errorf("change event value = %v, want %q", result.Events[0].Data["value"], "survey ")
	}
}

func TestInputWidget_Validation_ValidPattern(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "abc", "pattern": "^[a-z]+$"})
	w.Init(n)
	if !w.valid {
		t.Error("expected valid for 'abc' matching ^[a-z]+$")
	}
}

func TestInputWidget_Validation_InvalidPattern(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "123", "pattern": "^[a-z]+$"})
	w.Init(n)
	if w.valid {
		t.Error("expected invalid for '123' not matching ^[a-z]+$")
	}
}

func TestInputWidget_Validation_EmptyValueNoPattern(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{})
	w.Init(n)
	if !w.valid {
		t.Error("expected valid when no pattern is set")
	}
}

func TestInputWidget_Validation_BadRegex(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "test", "pattern": "([invalid"})
	w.Init(n)
	// Bad regex should not crash; treated as always valid.
	if !w.valid {
		t.Error("expected valid when pattern regex is invalid")
	}
}

func TestInputWidget_ExternalValueUpdate(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "old"})
	w.Init(n)

	// Simulate external prop change (via patch).
	n.SetProp("value", "new")
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "new") {
		t.Errorf("expected updated value in output, got %q", got)
	}
}

func TestInputWidget_LayoutReturnsNil(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("input Layout should return nil")
	}
}

func TestInputWidget_CursorMovement(t *testing.T) {
	w := &InputWidget{}
	n := inputNode(t, map[string]any{"value": "hello"})
	w.Init(n)

	// Move cursor left.
	w.Update(tea.KeyMsg{Type: tea.KeyLeft}, n)
	// Type a character — should insert before last character.
	w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}}, n)
	if w.value != "hellXo" {
		t.Errorf("after left+insert: value = %q, want %q", w.value, "hellXo")
	}
}
