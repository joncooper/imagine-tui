package widget

import (
	"strings"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/joncooper/imagine-tui/internal/testutil"
)

func sparklineNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("sparkline", dom.TypeSparkline)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestSparklineWidget_View_RendersSeries(t *testing.T) {
	w := &SparklineWidget{}
	n := sparklineNode(t, map[string]any{
		"label":  "Trend",
		"values": []any{1, 3, 2, 5, 8},
	})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 24, Theme: DefaultTheme()})
	if !strings.Contains(got, "Trend") {
		t.Fatalf("expected label in output, got:\n%s", got)
	}
	if !strings.ContainsAny(got, "▁▂▃▄▅▆▇█") {
		t.Fatalf("expected sparkline runes in output, got:\n%s", got)
	}
}

func TestSparklineWidget_View_Empty(t *testing.T) {
	w := &SparklineWidget{}
	n := sparklineNode(t, map[string]any{})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 24, Theme: DefaultTheme()})
	if !strings.Contains(got, "(empty)") {
		t.Fatalf("expected empty placeholder, got:\n%s", got)
	}
}

func TestSparklineWidget_View_ZeroWidth(t *testing.T) {
	w := &SparklineWidget{}
	n := sparklineNode(t, map[string]any{"values": []any{1, 2, 3}})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 0, Theme: DefaultTheme()})
	if got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}

func TestSparklineWidget_LayoutReturnsNil(t *testing.T) {
	w := &SparklineWidget{}
	n := sparklineNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Fatal("sparkline Layout should return nil")
	}
}

func TestSparklineWidget_GoldenFiles(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]any
		width int
	}{
		{
			name: "labeled",
			props: map[string]any{
				"label":  "CPU",
				"values": []any{12, 18, 22, 30, 26, 35, 42},
			},
			width: 24,
		},
		{
			name: "clipped",
			props: map[string]any{
				"values": []any{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
			width: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &SparklineWidget{}
			n := sparklineNode(t, tt.props)
			w.Init(n)

			got := w.View(n, nil, ViewContext{Width: tt.width, Theme: DefaultTheme()})
			testutil.GoldenFile(t, "widget/sparkline_"+tt.name+".golden", []byte(got))
		})
	}
}
