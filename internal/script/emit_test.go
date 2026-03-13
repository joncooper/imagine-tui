package script

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEmitLocalAppliesPatch(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `
		emit('local', [{op: 'update', id: 'child1', props: {text: 'patched'}}])
	`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	v, _ := node.GetProp("text")
	if v != "patched" {
		t.Errorf("expected 'patched', got %v", v)
	}
}

func TestEmitLocalMultipleOps(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `
		emit('local', [
			{op: 'update', id: 'child1', props: {text: 'one'}},
			{op: 'update', id: 'child2', props: {value: 'two'}}
		])
	`, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1 := rt.tree.Find("child1")
	v1, _ := n1.GetProp("text")
	if v1 != "one" {
		t.Errorf("child1.text: expected 'one', got %v", v1)
	}

	n2 := rt.tree.Find("child2")
	v2, _ := n2.GetProp("value")
	if v2 != "two" {
		t.Errorf("child2.value: expected 'two', got %v", v2)
	}
}

func TestEmitLocalInvalidOpsThrows(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `
		emit('local', [{op: 'update', id: 'nonexistent', props: {text: 'x'}}])
	`, nil)
	if err == nil {
		t.Fatal("expected error for patch on nonexistent node")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
}

func TestEmitLocalInsertNode(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `
		emit('local', [{op: 'insert', parent_id: 'root', id: 'new_node', type: 'text', props: {text: 'inserted'}}])
	`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("new_node")
	if node == nil {
		t.Fatal("expected new_node to exist")
	}
	v, _ := node.GetProp("text")
	if v != "inserted" {
		t.Errorf("expected 'inserted', got %v", v)
	}
}

func TestEmitClaudeEnqueuesEvent(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("child1", "test", `
		emit('claude', {action: 'submit', detail: 'test_data'})
	`, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Dequeue the event.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	evt, err := rt.events.Dequeue(ctx, nil)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}
	if evt.Type != "script" {
		t.Errorf("expected event type 'script', got %q", evt.Type)
	}
	if evt.Source != "child1" {
		t.Errorf("expected source 'child1', got %q", evt.Source)
	}
	if evt.Data["action"] != "submit" {
		t.Errorf("expected action 'submit', got %v", evt.Data["action"])
	}
	if evt.Data["detail"] != "test_data" {
		t.Errorf("expected detail 'test_data', got %v", evt.Data["detail"])
	}
}

func TestEmitClaudeWithNestedData(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `
		emit('claude', {items: [1, 2, 3], nested: {key: 'val'}})
	`, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	evt, err := rt.events.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	items, ok := evt.Data["items"].([]any)
	if !ok || len(items) != 3 {
		t.Errorf("expected items=[1,2,3], got %v", evt.Data["items"])
	}
}

func TestEmitInvalidTargetThrows(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `
		emit('bad_target', {})
	`, nil)
	if err == nil {
		t.Fatal("expected error for invalid emit target")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
}

func TestEmitTooFewArgsThrows(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.execScript("root", "test", `emit('local')`, nil)
	if err == nil {
		t.Fatal("expected error for too few args")
	}
}
