package script

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestExecHookOnMount(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Attach on_mount script to child1.
	node := rt.tree.Find("child1")
	node.Scripts["on_mount"] = `$.text = "mounted"`

	dirty, err := rt.ExecHook("child1", HookOnMount, nil)
	if err != nil {
		t.Fatal(err)
	}

	v, _ := node.GetProp("text")
	if v != "mounted" {
		t.Errorf("expected 'mounted', got %v", v)
	}

	// child1 should be in the dirty set.
	found := false
	for _, id := range dirty {
		if id == "child1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected child1 in dirty set")
	}
}

func TestExecHookOnChange(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Attach on_change that reads the event payload.
	node := rt.tree.Find("child2")
	node.Scripts["on_change"] = `$.text = "changed:" + event.Data.value`

	dirty, err := rt.ExecHook("child2", HookOnChange, &HookPayload{
		Data: map[string]any{"value": "new_val"},
	})
	if err != nil {
		t.Fatal(err)
	}

	v, _ := node.GetProp("text")
	if v != "changed:new_val" {
		t.Errorf("expected 'changed:new_val', got %v", v)
	}

	if len(dirty) == 0 {
		t.Error("expected dirty nodes")
	}
}

func TestExecHookOnKey(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	node := rt.tree.Find("child2")
	node.Scripts["on_key"] = `$.text = "key:" + event.Key`

	_, err := rt.ExecHook("child2", HookOnKey, &HookPayload{Key: "Enter"})
	if err != nil {
		t.Fatal(err)
	}

	v, _ := node.GetProp("text")
	if v != "key:Enter" {
		t.Errorf("expected 'key:Enter', got %v", v)
	}
}

func TestExecHookNoScriptIsNoop(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// child1 has no on_focus script. ExecHook should return empty dirty set and no error.
	dirty, err := rt.ExecHook("child1", HookOnFocus, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty nodes, got %v", dirty)
	}
}

func TestExecHookNodeNotFound(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	_, err := rt.ExecHook("nonexistent", HookOnMount, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
}

func TestExecHookTimeout(t *testing.T) {
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	events := dom.NewEventQueue()
	rt := New(tree, events, WithTimeout(50*time.Millisecond))

	root = rt.tree.Find("root")
	root.Scripts["on_mount"] = `while(true){}`

	_, err = rt.ExecHook("root", HookOnMount, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if !se.IsTimeout {
		t.Error("expected IsTimeout to be true")
	}
}

func TestExecHookModifiesDOMReturnsDirty(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Script modifies both child1 and child2.
	node := rt.tree.Find("root")
	node.Scripts["on_mount"] = `
		$('child1').text = "a";
		$('child2').value = "b";
	`

	dirty, err := rt.ExecHook("root", HookOnMount, nil)
	if err != nil {
		t.Fatal(err)
	}

	dirtyMap := make(map[string]bool)
	for _, id := range dirty {
		dirtyMap[id] = true
	}
	if !dirtyMap["child1"] {
		t.Error("expected child1 in dirty set")
	}
	if !dirtyMap["child2"] {
		t.Error("expected child2 in dirty set")
	}
}

func TestNotifyMount(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	node := rt.tree.Find("child1")
	node.Scripts["on_mount"] = `$.text = "mount_fired"`

	err := rt.NotifyMount("child1")
	if err != nil {
		t.Fatal(err)
	}

	v, _ := node.GetProp("text")
	if v != "mount_fired" {
		t.Errorf("expected 'mount_fired', got %v", v)
	}
}

func TestNotifyMountNoScript(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// No on_mount script — should be a no-op, not an error.
	err := rt.NotifyMount("child1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotifyRemoveCleansState(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Set some state.
	err := rt.execScript("child1", "test", `state.x = 42`, nil)
	if err != nil {
		t.Fatal(err)
	}

	rt.NotifyRemove("child1")

	// State should be gone.
	rt.mu.Lock()
	_, exists := rt.states["child1"]
	rt.mu.Unlock()

	if exists {
		t.Error("expected state to be removed after NotifyRemove")
	}
}

func TestNotifyChange(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	node := rt.tree.Find("child2")
	node.Scripts["on_change"] = `$.text = "val:" + $.value`

	err := rt.NotifyChange("child2", "new_value")
	if err != nil {
		t.Fatal(err)
	}

	// The value prop should have been set before the script ran.
	v, _ := node.GetProp("value")
	if v != "new_value" {
		t.Errorf("expected value='new_value', got %v", v)
	}

	// The script should have set text based on the new value.
	txt, _ := node.GetProp("text")
	if txt != "val:new_value" {
		t.Errorf("expected text='val:new_value', got %v", txt)
	}
}

func TestNotifyChangeNoScript(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// No on_change script. Value should still be updated.
	err := rt.NotifyChange("child2", "silent_update")
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child2")
	v, _ := node.GetProp("value")
	if v != "silent_update" {
		t.Errorf("expected 'silent_update', got %v", v)
	}
}

func TestNotifyChangeNodeNotFound(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	err := rt.NotifyChange("nonexistent", "x")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestExecHookEmitsClaude(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	node := rt.tree.Find("child1")
	node.Scripts["on_mount"] = `emit('claude', {action: 'mounted'})`

	_, err := rt.ExecHook("child1", HookOnMount, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	evt, err := rt.events.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "child1" {
		t.Errorf("expected source 'child1', got %q", evt.Source)
	}
	if evt.Data["action"] != "mounted" {
		t.Errorf("expected action 'mounted', got %v", evt.Data["action"])
	}
}

func TestHookTypesAllValid(t *testing.T) {
	// Verify all hook type constants are defined.
	hooks := []HookType{
		HookOnMount, HookOnChange, HookOnEvent,
		HookOnFocus, HookOnBlur, HookOnKey,
	}
	for _, h := range hooks {
		if h == "" {
			t.Error("empty hook type found")
		}
	}
}
