package widget

import (
	"strings"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func textNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("txt", dom.TypeText)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestTextWidget_PlainText(t *testing.T) {
	w := &TextWidget{}
	n := textNode(t, map[string]any{"text": "hello world"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected 'hello world' in output, got %q", got)
	}
}

func TestTextWidget_EmptyText(t *testing.T) {
	w := &TextWidget{}
	n := textNode(t, map[string]any{})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	// Empty text should not panic, may produce empty or minimal output.
	_ = got
}

func TestTextWidget_ZeroWidth(t *testing.T) {
	w := &TextWidget{}
	n := textNode(t, map[string]any{"text": "hello"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestTextWidget_WordWrap(t *testing.T) {
	w := &TextWidget{}
	longText := "This is a long text that should wrap at the given width boundary"
	n := textNode(t, map[string]any{"text": longText})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 20, Theme: DefaultTheme()})
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected text to wrap into multiple lines at width 20, got %d lines", len(lines))
	}
}

func TestTextWidget_WrapDisabled(t *testing.T) {
	w := &TextWidget{}
	longText := "This is a long text that should not wrap"
	n := textNode(t, map[string]any{"text": longText, "wrap": false})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 20, Theme: DefaultTheme()})
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Errorf("expected single line with wrap=false, got %d lines", len(lines))
	}
}

func TestTextWidget_MaxLines(t *testing.T) {
	w := &TextWidget{}
	longText := "Line one\nLine two\nLine three\nLine four\nLine five"
	n := textNode(t, map[string]any{"text": longText, "max_lines": 3})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	lines := strings.Split(got, "\n")
	if len(lines) > 3 {
		t.Errorf("expected at most 3 lines with max_lines=3, got %d", len(lines))
	}
}

func TestTextWidget_Segments(t *testing.T) {
	w := &TextWidget{}
	segments := []any{
		map[string]any{"text": "ERROR ", "style": "bold danger"},
		map[string]any{"text": "something broke", "style": ""},
	}
	n := textNode(t, map[string]any{"segments": segments})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "ERROR") || !strings.Contains(got, "something broke") {
		t.Errorf("expected both segments in output, got %q", got)
	}
}

func TestTextWidget_StyleTokens(t *testing.T) {
	w := &TextWidget{}
	n := textNode(t, map[string]any{"text": "styled", "style": "bold"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "styled") {
		t.Errorf("expected 'styled' in output, got %q", got)
	}
}

func TestTextWidget_LayoutReturnsNil(t *testing.T) {
	w := &TextWidget{}
	n := textNode(t, nil)
	got := w.Layout(n, ViewContext{Width: 40})
	if got != nil {
		t.Error("text widget Layout should return nil (not a container)")
	}
}

func TestTextWidget_UpdateNotConsumed(t *testing.T) {
	w := &TextWidget{}
	n := textNode(t, nil)
	result := w.Update(nil, n)
	if result.Consumed {
		t.Error("text widget should not consume events")
	}
}
