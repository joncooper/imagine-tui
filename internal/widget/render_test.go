package widget

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestWidgetTree_Render_LeafNode(t *testing.T) {
	r := NewRegistry()
	r.Register(dom.TypeText, func() Widget { return &stubWidget{} })

	root, _ := dom.NewNode("root", dom.TypeText)
	root.SetProp("text", "hello")
	tree, _ := dom.NewTree(root)

	wt := NewWidgetTree(r)
	_ = wt.Sync(tree)

	got := wt.Render(tree, 40, 0, "")
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestWidgetTree_Render_ZeroWidth(t *testing.T) {
	r := NewRegistry()
	r.Register(dom.TypeText, func() Widget { return &stubWidget{} })

	root, _ := dom.NewNode("root", dom.TypeText)
	tree, _ := dom.NewTree(root)

	wt := NewWidgetTree(r)
	_ = wt.Sync(tree)

	got := wt.Render(tree, 0, 0, "")
	if got != "" {
		t.Errorf("got %q, want empty string for zero width", got)
	}
}

func TestWidgetTree_Render_FocusPassedToWidget(t *testing.T) {
	var capturedFocused bool
	r := NewRegistry()
	r.Register(dom.TypeButton, func() Widget {
		return &focusCaptureWidget{captured: &capturedFocused}
	})

	root, _ := dom.NewNode("btn", dom.TypeButton)
	tree, _ := dom.NewTree(root)

	wt := NewWidgetTree(r)
	_ = wt.Sync(tree)

	_ = wt.Render(tree, 40, 0, "btn")
	if !capturedFocused {
		t.Error("expected focused=true to be passed to the widget")
	}
}

// focusCaptureWidget records whether it was rendered as focused.
type focusCaptureWidget struct {
	stubWidget
	captured *bool
}

func (w *focusCaptureWidget) View(node *dom.Node, children []RenderedChild, ctx ViewContext) string {
	*w.captured = ctx.Focused
	return "btn"
}
