package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func logNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("log", dom.TypeLog)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

var testLogLines = []any{
	map[string]any{"text": "Starting...", "level": "info"},
	map[string]any{"text": "Connected", "level": "info"},
	map[string]any{"text": "Warning: slow query", "level": "warn"},
	map[string]any{"text": "Error: timeout", "level": "error"},
	map[string]any{"text": "Debug info", "level": "debug"},
}

func TestLogWidget_View_ShowsLines(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{"lines": testLogLines})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "Starting...") || !strings.Contains(got, "Error: timeout") {
		t.Errorf("expected log lines in output, got:\n%s", got)
	}
}

func TestLogWidget_View_Empty(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{"lines": []any{}})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Errorf("expected empty placeholder, got:\n%s", got)
	}
}

func TestLogWidget_View_ZeroWidth(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{"lines": testLogLines})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestLogWidget_View_WithTimestamp(t *testing.T) {
	w := &LogWidget{}
	lines := []any{
		map[string]any{"text": "hello", "level": "info", "timestamp": "10:32:01"},
	}
	n := logNode(t, map[string]any{"lines": lines})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "10:32:01") {
		t.Errorf("expected timestamp in output, got:\n%s", got)
	}
}

func TestLogWidget_View_MaxLines(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{
		"lines":     testLogLines,
		"max_lines": 2,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	// Should only show last 2 lines (most recent).
	lineCount := strings.Count(got, "\n") + 1
	if lineCount > 3 { // allow some flexibility for styling
		t.Errorf("expected at most ~2 visible lines with max_lines=2, got %d", lineCount)
	}
}

func TestLogWidget_StickyBottom(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{
		"lines":       testLogLines,
		"auto_scroll": true,
	})
	w.Init(n)
	if !w.stickyBottom {
		t.Error("expected stickyBottom=true with auto_scroll=true")
	}
}

func TestLogWidget_ScrollUp_DisengagesSticky(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{
		"lines":       testLogLines,
		"auto_scroll": true,
	})
	w.Init(n)

	// Move away from top so scroll-up can actually scroll.
	w.scrollOffset = 3
	w.Update(tea.KeyMsg{Type: tea.KeyUp}, n)
	if w.stickyBottom {
		t.Error("expected stickyBottom=false after scrolling up from non-top position")
	}
}

func TestLogWidget_Update_ScrollDown(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, map[string]any{"lines": testLogLines})
	w.Init(n)
	w.scrollOffset = 0

	result := w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if !result.Consumed {
		t.Error("expected consumed")
	}
}

func TestLogWidget_LayoutReturnsNil(t *testing.T) {
	w := &LogWidget{}
	n := logNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("log Layout should return nil")
	}
}
