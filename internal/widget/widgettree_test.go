package widget

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func makeTestTree(t *testing.T) *dom.Tree {
	t.Helper()
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	child1, _ := dom.NewNode("c1", dom.TypeText)
	child2, _ := dom.NewNode("c2", dom.TypeButton)
	if err := tree.Insert("root", child1, ""); err != nil {
		t.Fatal(err)
	}
	if err := tree.Insert("root", child2, ""); err != nil {
		t.Fatal(err)
	}
	return tree
}

func testRegistry() *Registry {
	r := NewRegistry()
	r.Register(dom.TypeContainer, func() Widget { return &stubWidget{typeName: "container"} })
	r.Register(dom.TypeText, func() Widget { return &stubWidget{typeName: "text"} })
	r.Register(dom.TypeButton, func() Widget { return &stubWidget{typeName: "button"} })
	return r
}

func TestNewWidgetTree(t *testing.T) {
	wt := NewWidgetTree(testRegistry())
	if wt == nil {
		t.Fatal("NewWidgetTree returned nil")
	}
}

func TestWidgetTree_Sync_CreatesInstances(t *testing.T) {
	wt := NewWidgetTree(testRegistry())
	tree := makeTestTree(t)

	if err := wt.Sync(tree); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Should have 3 instances: root, c1, c2.
	for _, id := range []string{"root", "c1", "c2"} {
		if wt.Get(id) == nil {
			t.Errorf("expected widget instance for %q", id)
		}
	}
}

func TestWidgetTree_Sync_InitCalledOnNewInstances(t *testing.T) {
	wt := NewWidgetTree(testRegistry())
	tree := makeTestTree(t)
	_ = wt.Sync(tree)

	w := wt.Get("c1").(*stubWidget)
	if !w.initialized {
		t.Error("expected Init to be called on new widget")
	}
}

func TestWidgetTree_Sync_RemovesStaleInstances(t *testing.T) {
	wt := NewWidgetTree(testRegistry())
	tree := makeTestTree(t)
	_ = wt.Sync(tree)

	// Remove c2 from the DOM.
	_, _ = tree.Remove("c2")
	_ = wt.Sync(tree)

	if wt.Get("c2") != nil {
		t.Error("expected widget for removed node c2 to be cleaned up")
	}
	// c1 should still exist.
	if wt.Get("c1") == nil {
		t.Error("expected widget for c1 to still exist")
	}
}

func TestWidgetTree_Sync_PreservesExistingInstances(t *testing.T) {
	wt := NewWidgetTree(testRegistry())
	tree := makeTestTree(t)
	_ = wt.Sync(tree)

	w1 := wt.Get("c1")
	_ = wt.Sync(tree) // sync again, no changes
	w2 := wt.Get("c1")

	if w1 != w2 {
		t.Error("expected same widget instance after re-sync with no changes")
	}
}

func TestWidgetTree_Sync_UnregisteredType(t *testing.T) {
	r := NewRegistry()
	// Only register container, not text or button.
	r.Register(dom.TypeContainer, func() Widget { return &stubWidget{} })

	wt := NewWidgetTree(r)
	tree := makeTestTree(t)

	err := wt.Sync(tree)
	if err == nil {
		t.Fatal("expected error for unregistered node type")
	}
}

func TestWidgetTree_Get_Nonexistent(t *testing.T) {
	wt := NewWidgetTree(testRegistry())
	if wt.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent widget")
	}
}
