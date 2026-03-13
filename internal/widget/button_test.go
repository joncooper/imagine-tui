package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func buttonNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("btn", dom.TypeButton)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestButtonWidget_View_ShowsLabel(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, map[string]any{"label": "Submit"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "Submit") {
		t.Errorf("expected 'Submit' in output, got %q", got)
	}
}

func TestButtonWidget_View_ZeroWidth(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, map[string]any{"label": "OK"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestButtonWidget_Update_Enter_EmitsClick(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, map[string]any{"label": "OK"})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if len(result.Events) != 1 || result.Events[0].Type != "click" {
		t.Error("expected click event on Enter")
	}
}

func TestButtonWidget_Update_Space_EmitsClick(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, map[string]any{"label": "OK"})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeySpace}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
	if len(result.Events) != 1 || result.Events[0].Type != "click" {
		t.Error("expected click event on Space")
	}
}

func TestButtonWidget_Update_Disabled(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, map[string]any{"label": "OK", "disabled": true})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, n)
	if len(result.Events) > 0 {
		t.Error("expected no events when disabled")
	}
}

func TestButtonWidget_Update_OtherKey_NotConsumed(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, map[string]any{"label": "OK"})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyTab}, n)
	if result.Consumed {
		t.Error("expected Tab not consumed by button")
	}
}

func TestButtonWidget_LayoutReturnsNil(t *testing.T) {
	w := &ButtonWidget{}
	n := buttonNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("button Layout should return nil")
	}
}
