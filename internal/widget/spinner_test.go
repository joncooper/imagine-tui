package widget

import (
	"strings"
	"testing"

	bubblespinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/joncooper/imagine-tui/internal/testutil"
)

func spinnerNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("spinner", dom.TypeSpinner)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestSpinnerWidget_View_ShowsFrameAndLabel(t *testing.T) {
	w := &SpinnerWidget{}
	n := spinnerNode(t, map[string]any{
		"label": "Loading",
	})
	w.Init(n)

	got := w.View(n, nil, ViewContext{Width: 20, Theme: DefaultTheme()})
	if !strings.Contains(got, "Loading") {
		t.Fatalf("expected label in output, got:\n%s", got)
	}
	if !strings.ContainsAny(got, "|/-\\") {
		t.Fatalf("expected spinner frame in output, got:\n%s", got)
	}
}

func TestSpinnerWidget_CommandStartsWhenActive(t *testing.T) {
	w := &SpinnerWidget{}
	n := spinnerNode(t, map[string]any{"active": true})
	w.Init(n)

	cmd := w.Command(n)
	if cmd == nil {
		t.Fatal("expected initial spinner command")
	}
	if _, ok := cmd().(bubblespinner.TickMsg); !ok {
		t.Fatalf("expected spinner.TickMsg, got %T", cmd())
	}
}

func TestSpinnerWidget_Update_TickAdvancesFrame(t *testing.T) {
	w := &SpinnerWidget{}
	n := spinnerNode(t, map[string]any{"active": true})
	w.Init(n)

	before := w.model.View()
	result := w.Update(w.model.Tick(), n)
	after := w.model.View()
	if !result.Consumed {
		t.Fatal("expected spinner tick to be consumed")
	}
	if result.Cmd == nil {
		t.Fatal("expected follow-up spinner command")
	}
	if before == after {
		t.Fatalf("expected spinner frame to advance, before=%q after=%q", before, after)
	}
}

func TestSpinnerWidget_LayoutReturnsNil(t *testing.T) {
	w := &SpinnerWidget{}
	n := spinnerNode(t, nil)
	if w.Layout(n, ViewContext{}) != nil {
		t.Fatal("spinner Layout should return nil")
	}
}

func TestSpinnerWidget_GoldenFiles(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]any
		width int
	}{
		{
			name: "default",
			props: map[string]any{
				"label": "Loading",
			},
			width: 20,
		},
		{
			name: "inactive",
			props: map[string]any{
				"label":  "Idle",
				"active": false,
			},
			width: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &SpinnerWidget{}
			n := spinnerNode(t, tt.props)
			w.Init(n)

			got := w.View(n, nil, ViewContext{Width: tt.width, Theme: DefaultTheme()})
			testutil.GoldenFile(t, "widget/spinner_"+tt.name+".golden", []byte(got))
		})
	}
}
