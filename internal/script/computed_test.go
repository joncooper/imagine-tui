package script

import (
	"errors"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestEvalComputedBasic(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Set a computed prop on child1 that returns a static value.
	node := rt.tree.Find("child1")
	node.Computed["text"] = `return "computed_value"`

	val, err := rt.EvalComputed("child1", "text", node.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	if val != "computed_value" {
		t.Errorf("expected 'computed_value', got %v", val)
	}
}

func TestEvalComputedReadsCrossNode(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// child2 has value "world". Computed prop on child1 reads it.
	node := rt.tree.Find("child1")
	node.Computed["text"] = `return "hello " + $('child2').value`

	val, err := rt.EvalComputed("child1", "text", node.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	if val != "hello world" {
		t.Errorf("expected 'hello world', got %v", val)
	}
}

func TestEvalComputedReadsCurrentNodeState(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("child1", "test", `state.count = 2`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	node.Computed["text"] = `return "count:" + state.count`

	val, err := rt.EvalComputed("child1", "text", node.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	if val != "count:2" {
		t.Errorf("expected 'count:2', got %v", val)
	}
}

func TestEvalComputedReadsCrossNodeState(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("child2", "test", `state.count = 4`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	node.Computed["text"] = `return "count:" + $('child2').state.count`

	val, err := rt.EvalComputed("child1", "text", node.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	if val != "count:4" {
		t.Errorf("expected 'count:4', got %v", val)
	}
}

func TestEvalComputedDependencyTracking(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	node := rt.tree.Find("child1")
	node.Computed["text"] = `return $('child2').value`

	_, err := rt.EvalComputed("child1", "text", node.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}

	// Check that the dep graph recorded child1.text depends on child2.
	rt.mu.Lock()
	deps := rt.deps.Dependents("child2")
	rt.mu.Unlock()

	found := false
	for _, dk := range deps {
		if dk.NodeID == "child1" && dk.PropName == "text" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected child1.text to depend on child2")
	}
}

func TestEvalComputedReEvalOnChange(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Set up: child1.text depends on child2.value via computed prop.
	child1 := rt.tree.Find("child1")
	child1.Computed["text"] = `return "got:" + $('child2').value`

	// Initial evaluation.
	val, err := rt.EvalComputed("child1", "text", child1.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	if val != "got:world" {
		t.Errorf("initial: expected 'got:world', got %v", val)
	}
	child1.SetProp("text", val)

	// Change child2's value.
	child2 := rt.tree.Find("child2")
	child2.SetProp("value", "updated")

	// Mark child2 as dirty and propagate.
	rt.mu.Lock()
	rt.dirty = map[string]bool{"child2": true}
	rt.mu.Unlock()

	err = rt.PropagateChanges()
	if err != nil {
		t.Fatal(err)
	}

	// child1.text should have been re-evaluated.
	v, _ := child1.GetProp("text")
	if v != "got:updated" {
		t.Errorf("after propagation: expected 'got:updated', got %v", v)
	}
}

func TestEvalComputedReEvalOnCurrentNodePropChange(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	child2 := rt.tree.Find("child2")
	child2.Computed["text"] = `return "self:" + $.value`

	val, err := rt.EvalComputed("child2", "text", child2.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	child2.SetProp("text", val)

	child2.SetProp("value", "updated")

	rt.mu.Lock()
	rt.dirty = map[string]bool{"child2": true}
	rt.mu.Unlock()

	err = rt.PropagateChanges()
	if err != nil {
		t.Fatal(err)
	}

	v, _ := child2.GetProp("text")
	if v != "self:updated" {
		t.Errorf("after propagation: expected 'self:updated', got %v", v)
	}
}

func TestEvalComputedReEvalOnStateChange(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("child2", "test", `state.count = 1`, nil)
	if err != nil {
		t.Fatal(err)
	}

	child1 := rt.tree.Find("child1")
	child1.Computed["text"] = `return "count:" + $('child2').state.count`

	val, err := rt.EvalComputed("child1", "text", child1.Computed["text"])
	if err != nil {
		t.Fatal(err)
	}
	child1.SetProp("text", val)

	err = rt.execScript("child2", "test", `state.count = 2`, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = rt.PropagateChanges()
	if err != nil {
		t.Fatal(err)
	}

	v, _ := child1.GetProp("text")
	if v != "count:2" {
		t.Errorf("after propagation: expected 'count:2', got %v", v)
	}
}

func TestEvalComputedCycleDetection(t *testing.T) {
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := dom.NewNode("a", dom.TypeText)
	b, _ := dom.NewNode("b", dom.TypeText)
	_ = tree.Insert("root", a, "")
	_ = tree.Insert("root", b, "")

	events := dom.NewEventQueue()
	rt := New(tree, events)

	// a.text depends on b; b.text depends on a. This is a cycle.
	a = rt.tree.Find("a")
	a.Computed["text"] = `return $('b').text`
	b = rt.tree.Find("b")
	b.Computed["text"] = `return $('a').text`

	// Evaluate a first to establish its dependencies.
	_, _ = rt.EvalComputed("a", "text", a.Computed["text"])
	// Evaluate b to establish its dependencies.
	_, _ = rt.EvalComputed("b", "text", b.Computed["text"])

	// Now trigger propagation by dirtying 'a'.
	rt.mu.Lock()
	rt.dirty = map[string]bool{"a": true}
	rt.mu.Unlock()

	err = rt.PropagateChanges()
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if se.Message == "" {
		t.Error("expected non-empty error message for cycle")
	}
}

func TestEvalComputedExpressionError(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	_, err := rt.EvalComputed("child1", "text", `return null.foo`)
	if err == nil {
		t.Fatal("expected error for expression error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if se.NodeID != "child1" {
		t.Errorf("expected NodeID 'child1', got %q", se.NodeID)
	}
	if se.Hook != "text" {
		t.Errorf("expected Hook 'text', got %q", se.Hook)
	}
}

func TestEvalComputedNodeNotFound(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	_, err := rt.EvalComputed("nonexistent", "text", `return "x"`)
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestEvalComputedTransitiveDeps(t *testing.T) {
	// a.val depends on b.val, b.val depends on c.val.
	// Changing c should propagate through b to a.
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := dom.NewNode("a", dom.TypeText)
	b, _ := dom.NewNode("b", dom.TypeText)
	c, _ := dom.NewNode("c", dom.TypeText)
	c.SetProp("value", "original")
	_ = tree.Insert("root", a, "")
	_ = tree.Insert("root", b, "")
	_ = tree.Insert("root", c, "")

	events := dom.NewEventQueue()
	rt := New(tree, events)

	// Set up b.text as computed from c.value.
	b = rt.tree.Find("b")
	b.Computed["text"] = `return $('c').value`
	val, _ := rt.EvalComputed("b", "text", b.Computed["text"])
	b.SetProp("text", val)

	// Set up a.text as computed from b.text.
	a = rt.tree.Find("a")
	a.Computed["text"] = `return $('b').text`
	val, _ = rt.EvalComputed("a", "text", a.Computed["text"])
	a.SetProp("text", val)

	// Verify initial state.
	aText, _ := a.GetProp("text")
	if aText != "original" {
		t.Fatalf("initial a.text: expected 'original', got %v", aText)
	}

	// Change c.value and propagate.
	c = rt.tree.Find("c")
	c.SetProp("value", "changed")

	rt.mu.Lock()
	rt.dirty = map[string]bool{"c": true}
	rt.mu.Unlock()

	err = rt.PropagateChanges()
	if err != nil {
		t.Fatal(err)
	}

	bText, _ := b.GetProp("text")
	if bText != "changed" {
		t.Errorf("b.text: expected 'changed', got %v", bText)
	}
	aText, _ = a.GetProp("text")
	if aText != "changed" {
		t.Errorf("a.text: expected 'changed', got %v", aText)
	}
}

func TestEvalComputedDepGraphUpdatesOnReEval(t *testing.T) {
	// A computed prop that conditionally reads different nodes.
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	src, _ := dom.NewNode("src", dom.TypeText)
	src.SetProp("value", "a")
	alt, _ := dom.NewNode("alt", dom.TypeText)
	alt.SetProp("value", "alt_val")
	dest, _ := dom.NewNode("dest", dom.TypeText)
	_ = tree.Insert("root", src, "")
	_ = tree.Insert("root", alt, "")
	_ = tree.Insert("root", dest, "")

	events := dom.NewEventQueue()
	rt := New(tree, events)

	// dest.text reads src.value.
	dest = rt.tree.Find("dest")
	dest.Computed["text"] = `return $('src').value`
	val, _ := rt.EvalComputed("dest", "text", dest.Computed["text"])
	dest.SetProp("text", val)

	// Verify src is a dependency.
	rt.mu.Lock()
	deps := rt.deps.Dependents("src")
	rt.mu.Unlock()
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependent of src, got %d", len(deps))
	}

	// Now change the computed expression to read alt instead.
	dest.Computed["text"] = `return $('alt').value`
	val, _ = rt.EvalComputed("dest", "text", dest.Computed["text"])
	dest.SetProp("text", val)

	// src should no longer be a dependency; alt should be.
	rt.mu.Lock()
	srcDeps := rt.deps.Dependents("src")
	altDeps := rt.deps.Dependents("alt")
	rt.mu.Unlock()

	if len(srcDeps) != 0 {
		t.Errorf("expected 0 dependents of src after re-eval, got %d", len(srcDeps))
	}
	if len(altDeps) != 1 {
		t.Errorf("expected 1 dependent of alt, got %d", len(altDeps))
	}
}

func TestEvalAllComputed(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	child1 := rt.tree.Find("child1")
	child1.Computed["text"] = `return "computed1"`
	child2 := rt.tree.Find("child2")
	child2.Computed["text"] = `return "computed2"`

	err := rt.EvalAllComputed()
	if err != nil {
		t.Fatal(err)
	}

	v1, _ := child1.GetProp("text")
	if v1 != "computed1" {
		t.Errorf("child1.text: expected 'computed1', got %v", v1)
	}
	v2, _ := child2.GetProp("text")
	if v2 != "computed2" {
		t.Errorf("child2.text: expected 'computed2', got %v", v2)
	}
}
