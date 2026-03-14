package script

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestStatePersistsAcrossInvocations(t *testing.T) {
	rt := newTestRuntime(t)

	// First invocation: set state.counter = 1
	err := rt.execScript("root", "test", `state.counter = 1`, nil)
	if err != nil {
		t.Fatalf("first invocation: %v", err)
	}

	// Second invocation: increment state.counter
	err = rt.execScript("root", "test", `state.counter = state.counter + 1`, nil)
	if err != nil {
		t.Fatalf("second invocation: %v", err)
	}

	// Third invocation: verify state.counter == 2 by emitting to a global we can check
	err = rt.execScript("root", "test", `
		if (state.counter !== 2) {
			throw new Error("expected counter=2, got " + state.counter);
		}
	`, nil)
	if err != nil {
		t.Fatalf("verification: %v", err)
	}
}

func TestStateIsolatedBetweenNodes(t *testing.T) {
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	nodeA, err := dom.NewNode("a", dom.TypeText)
	if err != nil {
		t.Fatal(err)
	}
	nodeB, err := dom.NewNode("b", dom.TypeText)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Insert("root", nodeA, ""); err != nil {
		t.Fatal(err)
	}
	if err := tree.Insert("root", nodeB, ""); err != nil {
		t.Fatal(err)
	}

	events := dom.NewEventQueue()
	rt := New(tree, events)

	// Set state on node A.
	err = rt.execScript("a", "test", `state.x = "from_a"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Set state on node B.
	err = rt.execScript("b", "test", `state.x = "from_b"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Verify node A still has its own state.
	err = rt.execScript("a", "test", `
		if (state.x !== "from_a") {
			throw new Error("expected 'from_a', got " + state.x);
		}
	`, nil)
	if err != nil {
		t.Fatalf("node A isolation: %v", err)
	}

	// Verify node B still has its own state.
	err = rt.execScript("b", "test", `
		if (state.x !== "from_b") {
			throw new Error("expected 'from_b', got " + state.x);
		}
	`, nil)
	if err != nil {
		t.Fatalf("node B isolation: %v", err)
	}
}

func TestStateSurvivesDOMPropUpdate(t *testing.T) {
	rt := newTestRuntime(t)

	// Set state on root.
	err := rt.execScript("root", "test", `state.val = 42`, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Update the node's props via DOM (simulate a patch).
	node := rt.tree.Find("root")
	node.SetProp("text", "updated")

	// State should still be there.
	err = rt.execScript("root", "test", `
		if (state.val !== 42) {
			throw new Error("expected 42, got " + state.val);
		}
	`, nil)
	if err != nil {
		t.Fatalf("state after prop update: %v", err)
	}
}

func TestStateGoneAfterRemoveState(t *testing.T) {
	rt := newTestRuntime(t)

	// Set state.
	err := rt.execScript("root", "test", `state.val = 99`, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Remove state for "root".
	rt.removeState("root")

	// State should be fresh (empty).
	err = rt.execScript("root", "test", `
		if (state.val !== undefined) {
			throw new Error("expected undefined, got " + state.val);
		}
	`, nil)
	if err != nil {
		t.Fatalf("state after removeState: %v", err)
	}
}

func TestStateDefaultsToEmptyObject(t *testing.T) {
	rt := newTestRuntime(t)

	// On first access, state should be an empty object (no crash).
	err := rt.execScript("root", "test", `
		if (typeof state !== "object") {
			throw new Error("expected object, got " + typeof state);
		}
		// Nullish coalescing pattern from the spec
		state.x = state.x ?? 0;
		state.x++;
		if (state.x !== 1) {
			throw new Error("expected 1, got " + state.x);
		}
	`, nil)
	if err != nil {
		t.Fatalf("state defaults: %v", err)
	}
}

func TestStateWriteMarksCurrentNodeDirty(t *testing.T) {
	rt := newTestRuntime(t)

	rt.mu.Lock()
	rt.dirty = make(map[string]bool)
	rt.mu.Unlock()

	err := rt.execScript("root", "test", `state.count = 1`, nil)
	if err != nil {
		t.Fatal(err)
	}

	rt.mu.Lock()
	isDirty := rt.dirty["root"]
	rt.mu.Unlock()

	if !isDirty {
		t.Error("expected root to be in dirty set after state write")
	}
}
