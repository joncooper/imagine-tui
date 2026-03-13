package script

import (
	"context"
	"testing"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// TestIntegrationFullStack exercises the complete script runtime:
// DOM tree → scripts with hooks → computed props → emit → state.
func TestIntegrationFullStack(t *testing.T) {
	// Build a tree: root > header + form(qty_input, price_input, total, submit_btn)
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	header, _ := dom.NewNode("header", dom.TypeText)
	header.SetProp("text", "Order Form")
	_ = tree.Insert("root", header, "")

	form, _ := dom.NewNode("form", dom.TypeContainer)
	_ = tree.Insert("root", form, "")

	qty, _ := dom.NewNode("qty", dom.TypeInput)
	qty.SetProp("value", float64(2))
	// on_change script that updates state.
	qty.Scripts["on_change"] = `state.lastChange = "qty"`
	_ = tree.Insert("form", qty, "")

	price, _ := dom.NewNode("price", dom.TypeInput)
	price.SetProp("value", float64(10))
	_ = tree.Insert("form", price, "")

	total, _ := dom.NewNode("total", dom.TypeText)
	// Computed prop: total = qty * price.
	total.Computed["text"] = `return "Total: $" + ($('qty').value * $('price').value)`
	_ = tree.Insert("form", total, "")

	btn, _ := dom.NewNode("submit_btn", dom.TypeButton)
	btn.SetProp("label", "Submit")
	// on_mount script emits to Claude.
	btn.Scripts["on_mount"] = `emit('claude', {action: 'btn_ready'})`
	_ = tree.Insert("form", btn, "")

	events := dom.NewEventQueue()
	rt := New(tree, events, WithTimeout(100*time.Millisecond))

	// Step 1: Evaluate all computed props.
	err := rt.EvalAllComputed()
	if err != nil {
		t.Fatalf("EvalAllComputed: %v", err)
	}
	v, _ := total.GetProp("text")
	if v != "Total: $20" {
		t.Errorf("initial total: expected 'Total: $20', got %v", v)
	}

	// Step 2: Fire on_mount for submit button → should emit claude event.
	err = rt.NotifyMount("submit_btn")
	if err != nil {
		t.Fatalf("NotifyMount: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	evt, err := events.Dequeue(ctx, nil)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if evt.Data["action"] != "btn_ready" {
		t.Errorf("expected action 'btn_ready', got %v", evt.Data["action"])
	}

	// Step 3: Simulate user changing qty to 5.
	err = rt.NotifyChange("qty", float64(5))
	if err != nil {
		t.Fatalf("NotifyChange: %v", err)
	}

	// Verify state was updated by on_change script.
	err = rt.execScript("qty", "verify", `
		if (state.lastChange !== "qty") {
			throw new Error("expected lastChange='qty', got " + state.lastChange);
		}
	`, nil)
	if err != nil {
		t.Fatalf("state verification: %v", err)
	}

	// Step 4: Propagate the change to computed props.
	rt.mu.Lock()
	rt.dirty["qty"] = true
	rt.mu.Unlock()
	err = rt.PropagateChanges()
	if err != nil {
		t.Fatalf("PropagateChanges: %v", err)
	}

	v, _ = total.GetProp("text")
	if v != "Total: $50" {
		t.Errorf("updated total: expected 'Total: $50', got %v", v)
	}

	// Step 5: Use emit('local') to add a node from a script.
	err = rt.execScript("root", "test", `
		emit('local', [{
			op: 'insert',
			parent_id: 'form',
			id: 'status',
			type: 'text',
			props: {text: 'Order pending'}
		}])
	`, nil)
	if err != nil {
		t.Fatalf("emit local: %v", err)
	}
	status := tree.Find("status")
	if status == nil {
		t.Fatal("expected status node to exist")
	}
	sv, _ := status.GetProp("text")
	if sv != "Order pending" {
		t.Errorf("status text: expected 'Order pending', got %v", sv)
	}

	// Step 6: Verify cross-node access in a script.
	err = rt.execScript("header", "test", `
		var tot = $('total').text;
		if (tot !== "Total: $50") {
			throw new Error("cross-node: expected 'Total: $50', got " + tot);
		}
	`, nil)
	if err != nil {
		t.Fatalf("cross-node verification: %v", err)
	}

	// Step 7: Remove a node and verify state cleanup.
	rt.NotifyRemove("qty")
	rt.mu.Lock()
	_, qtyStateExists := rt.states["qty"]
	rt.mu.Unlock()
	if qtyStateExists {
		t.Error("expected qty state to be cleaned up after NotifyRemove")
	}
}
