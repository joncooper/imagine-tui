package widget

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestNewTree(t *testing.T) {
	wt := NewTree(testRegistry())
	if wt == nil {
		t.Fatal("NewTree returned nil")
	}
}

func TestTree_Sync_CreatesInstances(t *testing.T) {
	wt := NewTree(testRegistry())
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

func TestTree_Sync_InitCalledOnNewInstances(t *testing.T) {
	wt := NewTree(testRegistry())
	tree := makeTestTree(t)
	_ = wt.Sync(tree)

	w := wt.Get("c1").(*stubWidget)
	if !w.initialized {
		t.Error("expected Init to be called on new widget")
	}
}

func TestTree_Sync_RemovesStaleInstances(t *testing.T) {
	wt := NewTree(testRegistry())
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

func TestTree_Sync_PreservesExistingInstances(t *testing.T) {
	wt := NewTree(testRegistry())
	tree := makeTestTree(t)
	_ = wt.Sync(tree)

	w1 := wt.Get("c1")
	_ = wt.Sync(tree) // sync again, no changes
	w2 := wt.Get("c1")

	if w1 != w2 {
		t.Error("expected same widget instance after re-sync with no changes")
	}
}

func TestTree_Sync_UnregisteredType_SkipsAndContinues(t *testing.T) {
	r := NewRegistry()
	// Only register container, not text or button.
	r.Register(dom.TypeContainer, func() Widget { return &stubWidget{} })

	wt := NewTree(r)
	tree := makeTestTree(t)

	// Sync should succeed (skipping unknown types) rather than aborting.
	err := wt.Sync(tree)
	if err != nil {
		t.Fatalf("Sync should skip unknown types, got error: %v", err)
	}

	// Container (root) should have a widget instance.
	if wt.Get("root") == nil {
		t.Error("expected widget instance for root (registered type)")
	}
	// text and button nodes should be skipped — no widget instance.
	if wt.Get("c1") != nil {
		t.Error("expected nil widget for c1 (unregistered type)")
	}
	if wt.Get("c2") != nil {
		t.Error("expected nil widget for c2 (unregistered type)")
	}
}

func TestTree_Get_Nonexistent(t *testing.T) {
	wt := NewTree(testRegistry())
	if wt.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent widget")
	}
}

func TestTree_Commands_WrapsNodeScopedCommands(t *testing.T) {
	r := NewRegistry()
	r.Register(dom.TypeProgress, func() Widget {
		return &commandStubWidget{
			command: func() tea.Msg { return "tick" },
		}
	})

	root, _ := dom.NewNode("progress", dom.TypeProgress)
	tree, _ := dom.NewTree(root)

	wt := NewTree(r)
	if err := wt.Sync(tree); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	cmds := wt.Commands(tree)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}

	msg := cmds[0]()
	scoped, ok := msg.(CommandMsg)
	if !ok {
		t.Fatalf("expected CommandMsg, got %T", msg)
	}
	if scoped.NodeID != "progress" {
		t.Fatalf("NodeID = %q, want %q", scoped.NodeID, "progress")
	}
	if scoped.Msg != tea.Msg("tick") {
		t.Fatalf("Msg = %#v, want %#v", scoped.Msg, tea.Msg("tick"))
	}
}

type commandStubWidget struct {
	stubWidget
	command func() tea.Msg
}

func (w *commandStubWidget) Command(_ *dom.Node) tea.Cmd {
	if w.command == nil {
		return nil
	}
	return w.command
}
