package widget

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/joncooper/imagine-tui/internal/testutil"
)

func containerNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("container", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

// --- Layout tests ---

func TestContainerLayout_Vertical_DefaultFullWidth(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "vertical"})
	// Add 3 children (just need Children slice length for layout).
	for i := 0; i < 3; i++ {
		child, _ := dom.NewNode("c"+string(rune('0'+i)), dom.TypeText)
		n.Children = append(n.Children, child)
	}

	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	if len(constraints) != 3 {
		t.Fatalf("expected 3 constraints, got %d", len(constraints))
	}
	for i, c := range constraints {
		if c.Width != 80 {
			t.Errorf("child %d: width = %d, want 80", i, c.Width)
		}
	}
}

func TestContainerLayout_Horizontal_EqualFill(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "horizontal"})
	// 3 children, no width specs → equal split.
	for i := 0; i < 3; i++ {
		child, _ := dom.NewNode("c"+string(rune('0'+i)), dom.TypeText)
		n.Children = append(n.Children, child)
	}

	ctx := ViewContext{Width: 90, Height: 24}
	constraints := w.Layout(n, ctx)

	if len(constraints) != 3 {
		t.Fatalf("expected 3 constraints, got %d", len(constraints))
	}
	// 90 / 3 = 30 each.
	for i, c := range constraints {
		if c.Width != 30 {
			t.Errorf("child %d: width = %d, want 30", i, c.Width)
		}
	}
}

func TestContainerLayout_Horizontal_WithGap(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "horizontal", "gap": 2})
	for i := 0; i < 3; i++ {
		child, _ := dom.NewNode("c"+string(rune('0'+i)), dom.TypeText)
		n.Children = append(n.Children, child)
	}

	// 90 - 2*2(gaps) = 86, 86/3 = 28 with 2 remainder.
	ctx := ViewContext{Width: 90, Height: 24}
	constraints := w.Layout(n, ctx)

	totalWidth := 0
	for _, c := range constraints {
		totalWidth += c.Width
	}
	// Total child widths + gaps should not exceed available.
	totalWithGaps := totalWidth + 2*2
	if totalWithGaps > 90 {
		t.Errorf("total with gaps = %d, exceeds available 90", totalWithGaps)
	}
}

func TestContainerLayout_Horizontal_FixedWidth(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "horizontal"})

	c1, _ := dom.NewNode("c1", dom.TypeText)
	c1.SetProp("width", 20)
	c2, _ := dom.NewNode("c2", dom.TypeText)
	// c2 gets remaining space.
	n.Children = append(n.Children, c1, c2)

	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	if constraints[0].Width != 20 {
		t.Errorf("c1 width = %d, want 20", constraints[0].Width)
	}
	if constraints[1].Width != 60 {
		t.Errorf("c2 width = %d, want 60", constraints[1].Width)
	}
}

func TestContainerLayout_Horizontal_PercentageWidth(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "horizontal"})

	c1, _ := dom.NewNode("c1", dom.TypeText)
	c1.SetProp("width", "25%")
	c2, _ := dom.NewNode("c2", dom.TypeText)
	c2.SetProp("width", "75%")
	n.Children = append(n.Children, c1, c2)

	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	if constraints[0].Width != 20 {
		t.Errorf("c1 width = %d, want 20 (25%% of 80)", constraints[0].Width)
	}
	if constraints[1].Width != 60 {
		t.Errorf("c2 width = %d, want 60 (75%% of 80)", constraints[1].Width)
	}
}

func TestContainerLayout_Horizontal_FillWidth(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "horizontal"})

	c1, _ := dom.NewNode("c1", dom.TypeText)
	c1.SetProp("width", 20)
	c2, _ := dom.NewNode("c2", dom.TypeText)
	c2.SetProp("width", "fill")
	c3, _ := dom.NewNode("c3", dom.TypeText)
	c3.SetProp("width", "fill")
	n.Children = append(n.Children, c1, c2, c3)

	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	if constraints[0].Width != 20 {
		t.Errorf("c1 width = %d, want 20", constraints[0].Width)
	}
	// Remaining 60 split between 2 fill children = 30 each.
	if constraints[1].Width != 30 {
		t.Errorf("c2 width = %d, want 30", constraints[1].Width)
	}
	if constraints[2].Width != 30 {
		t.Errorf("c3 width = %d, want 30", constraints[2].Width)
	}
}

func TestContainerLayout_WithPadding(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{
		"direction": "horizontal",
		"padding":   2,
	})
	c1, _ := dom.NewNode("c1", dom.TypeText)
	n.Children = append(n.Children, c1)

	// 80 - 2*2 padding = 76 available.
	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	if constraints[0].Width != 76 {
		t.Errorf("c1 width = %d, want 76 (80 - 4 padding)", constraints[0].Width)
	}
}

func TestContainerLayout_NoChildren(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, nil)
	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)
	if constraints != nil {
		t.Errorf("expected nil constraints for no children, got %v", constraints)
	}
}

// --- View tests ---

func TestContainerView_Vertical_JoinsChildren(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "vertical"})

	children := []RenderedChild{
		{NodeID: "c1", View: "line1"},
		{NodeID: "c2", View: "line2"},
	}
	ctx := ViewContext{Width: 40, Height: 0, Theme: DefaultTheme()}
	got := w.View(n, children, ctx)

	if !strings.Contains(got, "line1") || !strings.Contains(got, "line2") {
		t.Errorf("expected both children in output, got:\n%s", got)
	}
}

func TestContainerView_Horizontal_JoinsChildren(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "horizontal"})

	children := []RenderedChild{
		{NodeID: "c1", View: "left"},
		{NodeID: "c2", View: "right"},
	}
	ctx := ViewContext{Width: 40, Height: 0, Theme: DefaultTheme()}
	got := w.View(n, children, ctx)

	if !strings.Contains(got, "left") || !strings.Contains(got, "right") {
		t.Errorf("expected both children in output, got:\n%s", got)
	}
}

func TestContainerView_WithBorder(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{
		"direction": "vertical",
		"border":    "rounded",
	})

	children := []RenderedChild{
		{NodeID: "c1", View: "hello"},
	}
	ctx := ViewContext{Width: 40, Height: 0, Theme: DefaultTheme()}
	got := w.View(n, children, ctx)

	// Rounded border uses ╭ ╮ ╰ ╯ characters.
	if !strings.Contains(got, "╭") {
		t.Errorf("expected rounded border character, got:\n%s", got)
	}
}

func TestContainerView_ZeroWidth(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, nil)
	ctx := ViewContext{Width: 0, Height: 0, Theme: DefaultTheme()}
	got := w.View(n, nil, ctx)
	if got != "" {
		t.Errorf("expected empty for zero width, got %q", got)
	}
}

func TestContainerView_Empty(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{"direction": "vertical"})
	ctx := ViewContext{Width: 40, Height: 0, Theme: DefaultTheme()}
	got := w.View(n, nil, ctx)
	// Empty container should render something (possibly empty string).
	// Just verify it doesn't panic.
	_ = got
}

func TestContainerView_OverflowScroll_ClipsToHeight(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{
		"direction": "vertical",
		"height":    3,
		"overflow":  "scroll",
	})

	children := []RenderedChild{
		{NodeID: "c1", View: "line1\nline2\nline3\nline4\nline5"},
	}
	ctx := ViewContext{Width: 5, Height: 10, Theme: DefaultTheme()}

	got := w.View(n, children, ctx)

	if strings.Contains(got, "line4") || strings.Contains(got, "line5") {
		t.Fatalf("expected clipped output, got:\n%s", got)
	}

	testutil.GoldenFile(t, "widget/container_overflow_initial.golden", []byte(got+"\n"))
}

func TestContainerUpdate_OverflowScroll_ScrollsViewport(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{
		"direction": "vertical",
		"height":    3,
		"overflow":  "scroll",
	})

	children := []RenderedChild{
		{NodeID: "c1", View: "line1\nline2\nline3\nline4\nline5"},
	}
	ctx := ViewContext{Width: 5, Height: 10, Theme: DefaultTheme()}

	_ = w.View(n, children, ctx)
	result := w.Update(tea.KeyMsg{Type: tea.KeyDown}, n)
	if !result.Consumed {
		t.Fatal("expected scroll key to be consumed")
	}

	got := w.View(n, children, ctx)
	if !strings.Contains(got, "line4") || strings.Contains(got, "line1") {
		t.Fatalf("expected viewport to scroll, got:\n%s", got)
	}

	testutil.GoldenFile(t, "widget/container_overflow_scrolled.golden", []byte(got+"\n"))
}

func TestContainerLayout_WithBorder_ReducesAvailable(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{
		"direction": "horizontal",
		"border":    "rounded",
	})
	c1, _ := dom.NewNode("c1", dom.TypeText)
	n.Children = append(n.Children, c1)

	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	// Rounded border is 1 char on each side = 2 total.
	if constraints[0].Width != 78 {
		t.Errorf("c1 width = %d, want 78 (80 - 2 border)", constraints[0].Width)
	}
}

func TestContainerLayout_Horizontal_Mixed(t *testing.T) {
	w := &ContainerWidget{}
	n := containerNode(t, map[string]any{
		"direction": "horizontal",
		"gap":       1,
	})

	c1, _ := dom.NewNode("c1", dom.TypeText)
	c1.SetProp("width", 10) // fixed
	c2, _ := dom.NewNode("c2", dom.TypeText)
	c2.SetProp("width", "25%") // percentage
	c3, _ := dom.NewNode("c3", dom.TypeText)
	c3.SetProp("width", "fill") // fill
	n.Children = append(n.Children, c1, c2, c3)

	// Available = 80 - 2 gaps = 78.
	// Fixed: 10. Percentage: 25% of 78 = 19. Fill: 78 - 10 - 19 = 49.
	ctx := ViewContext{Width: 80, Height: 24}
	constraints := w.Layout(n, ctx)

	if constraints[0].Width != 10 {
		t.Errorf("c1 width = %d, want 10", constraints[0].Width)
	}
	if constraints[1].Width != 19 {
		t.Errorf("c2 width = %d, want 19 (25%% of 78)", constraints[1].Width)
	}
	if constraints[2].Width != 49 {
		t.Errorf("c3 width = %d, want 49 (78-10-19)", constraints[2].Width)
	}
}
