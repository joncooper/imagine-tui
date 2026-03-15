package widget

import (
	"strings"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestDefaultRegistry_AllV1Types(t *testing.T) {
	r := DefaultRegistry()
	v1Types := []dom.NodeType{
		dom.TypeContainer, dom.TypeText, dom.TypeInput, dom.TypeTextarea,
		dom.TypeSelect, dom.TypeButton, dom.TypeTable, dom.TypeList,
		dom.TypeDiff, dom.TypeLog, dom.TypeCode, dom.TypeProgress, dom.TypeSpinner,
		dom.TypeMarkdown, dom.TypeSparkline,
	}
	for _, nt := range v1Types {
		if !r.Has(nt) {
			t.Errorf("default registry missing widget for type %q", nt)
		}
	}
}

func TestIntegration_FullRenderPipeline(t *testing.T) {
	// Build a DOM tree: container > text + button.
	root, _ := dom.NewNode("root", dom.TypeContainer)
	root.SetProp("direction", "vertical")
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}

	header, _ := dom.NewNode("header", dom.TypeText)
	header.SetProp("text", "Welcome")
	_ = tree.Insert("root", header, "")

	btn, _ := dom.NewNode("action", dom.TypeButton)
	btn.SetProp("label", "Click me")
	_ = tree.Insert("root", btn, "")

	// Create Tree and render.
	r := DefaultRegistry()
	wt := NewTree(r)
	if err := wt.Sync(tree); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := wt.Render(tree, 60, 24, "action")
	if !strings.Contains(got, "Welcome") {
		t.Errorf("expected 'Welcome' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Click me") {
		t.Errorf("expected 'Click me' in output, got:\n%s", got)
	}
}

func TestIntegration_NestedContainers(t *testing.T) {
	// root(horizontal) > left(vertical) > text + right(vertical) > text
	root, _ := dom.NewNode("root", dom.TypeContainer)
	root.SetProp("direction", "horizontal")
	tree, _ := dom.NewTree(root)

	left, _ := dom.NewNode("left", dom.TypeContainer)
	left.SetProp("direction", "vertical")
	left.SetProp("width", "50%")
	_ = tree.Insert("root", left, "")

	right, _ := dom.NewNode("right", dom.TypeContainer)
	right.SetProp("direction", "vertical")
	right.SetProp("width", "50%")
	_ = tree.Insert("root", right, "")

	lt, _ := dom.NewNode("lt", dom.TypeText)
	lt.SetProp("text", "LEFT")
	_ = tree.Insert("left", lt, "")

	rt, _ := dom.NewNode("rt", dom.TypeText)
	rt.SetProp("text", "RIGHT")
	_ = tree.Insert("right", rt, "")

	r := DefaultRegistry()
	wt := NewTree(r)
	_ = wt.Sync(tree)

	got := wt.Render(tree, 80, 24, "")
	if !strings.Contains(got, "LEFT") || !strings.Contains(got, "RIGHT") {
		t.Errorf("expected both panels in output, got:\n%s", got)
	}
}

func TestIntegration_TableInContainer(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	root.SetProp("direction", "vertical")
	tree, _ := dom.NewTree(root)

	header, _ := dom.NewNode("header", dom.TypeText)
	header.SetProp("text", "Test Results")
	header.SetProp("style", "bold")
	_ = tree.Insert("root", header, "")

	tbl, _ := dom.NewNode("results", dom.TypeTable)
	tbl.SetProp("columns", []any{
		map[string]any{"key": "name", "label": "Test"},
		map[string]any{"key": "status", "label": "Status"},
	})
	tbl.SetProp("rows", []any{
		map[string]any{"name": "test_login", "status": "pass"},
		map[string]any{"name": "test_signup", "status": "fail"},
	})
	_ = tree.Insert("root", tbl, "")

	r := DefaultRegistry()
	wt := NewTree(r)
	_ = wt.Sync(tree)

	got := wt.Render(tree, 60, 24, "results")
	if !strings.Contains(got, "Test Results") {
		t.Errorf("expected header in output")
	}
	if !strings.Contains(got, "test_login") || !strings.Contains(got, "test_signup") {
		t.Errorf("expected table rows in output, got:\n%s", got)
	}
}

func TestIntegration_SyncAfterPatch(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	txt, _ := dom.NewNode("msg", dom.TypeText)
	txt.SetProp("text", "before")
	_ = tree.Insert("root", txt, "")

	r := DefaultRegistry()
	wt := NewTree(r)
	_ = wt.Sync(tree)

	got := wt.Render(tree, 40, 0, "")
	if !strings.Contains(got, "before") {
		t.Error("expected 'before' in initial render")
	}

	// Apply a patch to change the text.
	_ = tree.Patch([]dom.PatchOp{
		{Op: dom.OpUpdate, ID: "msg", Props: map[string]any{"text": "after"}},
	})

	// Re-sync and re-render.
	_ = wt.Sync(tree)
	got = wt.Render(tree, 40, 0, "")
	if !strings.Contains(got, "after") {
		t.Errorf("expected 'after' in patched render, got:\n%s", got)
	}
}
