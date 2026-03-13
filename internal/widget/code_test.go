package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

func codeNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("code", dom.TypeCode)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

const testGoCode = `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`

func TestCodeWidget_View_ShowsContent(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{"content": testGoCode, "language": "go"})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "package") || !strings.Contains(got, "main") {
		t.Errorf("expected code content in output, got:\n%s", got)
	}
}

func TestCodeWidget_View_LineNumbers(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{
		"content":      "line1\nline2\nline3",
		"line_numbers": true,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "1") || !strings.Contains(got, "3") {
		t.Errorf("expected line numbers in output, got:\n%s", got)
	}
}

func TestCodeWidget_View_NoLineNumbers(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{
		"content":      "line1\nline2",
		"line_numbers": false,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	// Without line numbers, should not have the gutter separator.
	if strings.Contains(got, "│") {
		t.Errorf("expected no gutter separator without line numbers, got:\n%s", got)
	}
}

func TestCodeWidget_View_StartLine(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{
		"content":      "line1\nline2\nline3",
		"line_numbers": true,
		"start_line":   42,
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "42") {
		t.Errorf("expected line number 42 in output, got:\n%s", got)
	}
}

func TestCodeWidget_View_HighlightLines(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{
		"content":         "line1\nline2\nline3\nline4",
		"highlight_lines": []any{2, 3},
	})
	w.Init(n)
	// Just verify it doesn't panic. Visual testing is better done via golden files.
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "line2") {
		t.Errorf("expected highlighted line content, got:\n%s", got)
	}
}

func TestCodeWidget_View_ZeroWidth(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{"content": testGoCode})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestCodeWidget_View_Empty(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Errorf("expected empty placeholder, got:\n%s", got)
	}
}

func TestCodeWidget_Update_Scroll(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{"content": testGoCode})
	w.Init(n)

	result := w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if !result.Consumed {
		t.Error("expected scroll consumed")
	}
	if w.scrollOffset != 1 {
		t.Errorf("scrollOffset = %d, want 1", w.scrollOffset)
	}
}

func TestCodeWidget_Update_ScrollUpAtTop(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{"content": testGoCode})
	w.Init(n)

	w.Update(tea.KeyMsg{Type: tea.KeyUp}, n)
	if w.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0 (clamped)", w.scrollOffset)
	}
}

func TestCodeWidget_LayoutReturnsNil(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Error("code Layout should return nil")
	}
}

func TestCodeWidget_View_HighlightRange(t *testing.T) {
	w := &CodeWidget{}
	n := codeNode(t, map[string]any{
		"content":         "a\nb\nc\nd\ne\nf\ng",
		"highlight_lines": []any{"3-5"},
	})
	w.Init(n)
	got := w.View(n, nil, ViewContext{Width: 80, Theme: DefaultTheme()})
	// Just verify no panic with range syntax.
	_ = got
}
