package widget

import (
	"strings"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/joncooper/imagine-tui/internal/testutil"
)

func markdownNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("markdown", dom.TypeMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestMarkdownWidget_View_RendersMarkdown(t *testing.T) {
	w := &MarkdownWidget{}
	n := markdownNode(t, map[string]any{
		"content": "# Title\n\n- alpha\n- beta",
	})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "Title") {
		t.Fatalf("expected heading in output, got:\n%s", got)
	}
	if !strings.Contains(got, "alpha") {
		t.Fatalf("expected list content in output, got:\n%s", got)
	}
}

func TestMarkdownWidget_View_Empty(t *testing.T) {
	w := &MarkdownWidget{}
	n := markdownNode(t, map[string]any{})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 40, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Fatalf("expected empty placeholder, got:\n%s", got)
	}
}

func TestMarkdownWidget_View_ZeroWidth(t *testing.T) {
	w := &MarkdownWidget{}
	n := markdownNode(t, map[string]any{"content": "# Title"})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}

func TestMarkdownWidget_LayoutReturnsNil(t *testing.T) {
	w := &MarkdownWidget{}
	n := markdownNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Fatal("markdown Layout should return nil")
	}
}

func TestMarkdownWidget_GoldenFiles(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]any
		width int
	}{
		{
			name: "basic",
			props: map[string]any{
				"content": "# Release Notes\n\n- Added progress\n- Added markdown",
			},
			width: 44,
		},
		{
			name: "code_block",
			props: map[string]any{
				"content": "## Example\n\n```go\nfmt.Println(\"hi\")\n```",
			},
			width: 44,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &MarkdownWidget{}
			n := markdownNode(t, tt.props)
			w.Init(n)

			got := w.View(n, nil, ViewContext{Width: tt.width, Theme: DefaultTheme()})
			testutil.GoldenFile(t, "widget/markdown_"+tt.name+".golden", []byte(got))
		})
	}
}
