package widget

import (
	"strings"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/joncooper/imagine-tui/internal/testutil"
)

func progressNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("progress", dom.TypeProgress)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestProgressWidget_View_ShowsLabelAndPercentage(t *testing.T) {
	w := &ProgressWidget{}
	n := progressNode(t, map[string]any{
		"label": "Build",
		"value": 42,
	})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 32, Theme: DefaultTheme()})
	if !strings.Contains(got, "Build") {
		t.Fatalf("expected label in output, got:\n%s", got)
	}
	if !strings.Contains(got, "42%") {
		t.Fatalf("expected percentage in output, got:\n%s", got)
	}
}

func TestProgressWidget_View_ClampsValue(t *testing.T) {
	w := &ProgressWidget{}
	n := progressNode(t, map[string]any{"value": 150})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 20, Theme: DefaultTheme()})
	if !strings.Contains(got, "100%") {
		t.Fatalf("expected clamped percentage in output, got:\n%s", got)
	}
}

func TestProgressWidget_View_ZeroWidth(t *testing.T) {
	w := &ProgressWidget{}
	n := progressNode(t, map[string]any{"value": 50})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}

func TestProgressWidget_LayoutReturnsNil(t *testing.T) {
	w := &ProgressWidget{}
	n := progressNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Fatal("progress Layout should return nil")
	}
}

func TestProgressWidget_GoldenFiles(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]any
		width int
	}{
		{
			name: "default",
			props: map[string]any{
				"label": "Deploy",
				"value": 42,
			},
			width: 32,
		},
		{
			name: "no_percent",
			props: map[string]any{
				"label":        "Deploy",
				"value":        42,
				"show_percent": false,
			},
			width: 28,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &ProgressWidget{}
			n := progressNode(t, tt.props)
			w.Init(n)

			got := w.View(n, nil, ViewContext{Width: tt.width, Theme: DefaultTheme()})
			testutil.GoldenFile(t, "widget/progress_"+tt.name+".golden", []byte(got))
		})
	}
}
