package script

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestCrossNodeReadText(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// Script runs on child2 but reads child1's text via $('child1').
	err := rt.execScript("child2", "test", `
		var txt = $('child1').text;
		if (txt !== "hello") {
			throw new Error("expected 'hello', got " + txt);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCrossNodeWriteText(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// Script runs on child2 but writes to child1.
	err := rt.execScript("child2", "test", `$('child1').text = "modified"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	v, _ := node.GetProp("text")
	if v != "modified" {
		t.Errorf("expected 'modified', got %v", v)
	}
}

func TestCrossNodeMissingReturnsUndefined(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("root", "test", `
		var proxy = $('nonexistent');
		if (proxy.text !== undefined) {
			throw new Error("expected undefined, got " + proxy.text);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCrossNodeMissingWriteNoCrash(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// Writing to a nonexistent node's proxy should not crash.
	err := rt.execScript("root", "test", `
		$('nonexistent').text = "should not crash";
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCrossNodeChainedAccess(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Set values on both children.
	node1 := rt.tree.Find("child1")
	node1.SetProp("value", float64(10))
	node2 := rt.tree.Find("child2")
	node2.SetProp("value", float64(20))

	err := rt.execScript("root", "test", `
		var sum = $('child1').value + $('child2').value;
		if (sum !== 30) {
			throw new Error("expected 30, got " + sum);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCurrentNodePropsStillWorkWithCallable(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// After making $ callable, $.id should still work on the current node.
	err := rt.execScript("child1", "test", `
		if ($.id !== "child1") {
			throw new Error("expected 'child1', got " + $.id);
		}
		if ($.text !== "hello") {
			throw new Error("expected 'hello', got " + $.text);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCurrentNodeWriteStillWorksWithCallable(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `$.text = "updated_via_callable"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	v, _ := node.GetProp("text")
	if v != "updated_via_callable" {
		t.Errorf("expected 'updated_via_callable', got %v", v)
	}
}

func TestCrossNodeReadID(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("root", "test", `
			if ($('child1').id !== "child1") {
				throw new Error("expected 'child1', got " + $('child1').id);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCrossNodeDirtiesTarget(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	rt.mu.Lock()
	rt.dirty = make(map[string]bool)
	rt.mu.Unlock()

	err := rt.execScript("root", "test", `$('child1').text = "dirty_check"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	rt.mu.Lock()
	isDirty := rt.dirty["child1"]
	rt.mu.Unlock()

	if !isDirty {
		t.Error("expected child1 to be in dirty set after cross-node write")
	}
}

func TestCrossNodeReadState(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("child1", "test", `state.count = 3`, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = rt.execScript("child2", "test", `
			if ($('child1').state.count !== 3) {
				throw new Error("expected 3, got " + $('child1').state.count);
			}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCrossNodeStateWriteMarksTargetDirty(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	rt.mu.Lock()
	rt.dirty = make(map[string]bool)
	rt.mu.Unlock()

	err := rt.execScript("root", "test", `$('child1').state.count = 7`, nil)
	if err != nil {
		t.Fatal(err)
	}

	rt.mu.Lock()
	isDirty := rt.dirty["child1"]
	rt.mu.Unlock()

	if !isDirty {
		t.Error("expected child1 to be in dirty set after cross-node state write")
	}
}

func TestCrossNodeDifferentTrees(t *testing.T) {
	// A more complex tree: root > panel > item1, item2
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	panel, _ := dom.NewNode("panel", dom.TypeContainer)
	_ = tree.Insert("root", panel, "")
	item1, _ := dom.NewNode("item1", dom.TypeText)
	item1.SetProp("text", "first")
	_ = tree.Insert("panel", item1, "")
	item2, _ := dom.NewNode("item2", dom.TypeText)
	item2.SetProp("text", "second")
	_ = tree.Insert("panel", item2, "")

	events := dom.NewEventQueue()
	rt := New(tree, events)

	// From item1, reach item2 (not a sibling in the same parent, but a tree peer).
	err = rt.execScript("item1", "test", `
		var t = $('item2').text;
		if (t !== "second") {
			throw new Error("expected 'second', got " + t);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}
