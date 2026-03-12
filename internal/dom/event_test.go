package dom

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestEventQueueBasicEnqueueDequeue(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{
		Type:   "click",
		Source: "btn1",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	evt, err := q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Type != "click" {
		t.Errorf("type = %q, want click", evt.Type)
	}
	if evt.Source != "btn1" {
		t.Errorf("source = %q, want btn1", evt.Source)
	}
}

func TestEventQueueFIFOOrder(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{Type: "click", Source: "btn1"})
	q.Enqueue(&Event{Type: "change", Source: "input1"})
	q.Enqueue(&Event{Type: "submit", Source: "form1"})

	ctx := context.Background()
	for _, expected := range []string{"btn1", "input1", "form1"} {
		evt, err := q.Dequeue(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if evt.Source != expected {
			t.Errorf("source = %q, want %q", evt.Source, expected)
		}
	}
}

func TestEventQueueBlocksUntilEvent(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	done := make(chan *Event, 1)
	go func() {
		ctx := context.Background()
		evt, _ := q.Dequeue(ctx, nil)
		done <- evt
	}()

	// Give goroutine time to block.
	time.Sleep(20 * time.Millisecond)

	q.Enqueue(&Event{Type: "click", Source: "btn1"})

	select {
	case evt := <-done:
		if evt.Source != "btn1" {
			t.Errorf("source = %q", evt.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("dequeue didn't unblock")
	}
}

func TestEventQueueTimeout(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	evt, err := q.Dequeue(ctx, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if evt != nil {
		t.Error("expected nil event on timeout")
	}
}

func TestEventQueueFilter(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{Type: "click", Source: "btn1"})
	q.Enqueue(&Event{Type: "click", Source: "btn2"})
	q.Enqueue(&Event{Type: "click", Source: "btn1"})

	ctx := context.Background()
	// Filter: only accept events from btn2.
	opts := &DequeueOpts{Filter: []string{"btn2"}}

	evt, err := q.Dequeue(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "btn2" {
		t.Errorf("source = %q, want btn2", evt.Source)
	}

	// The btn1 events should still be in the queue for an unfiltered dequeue.
	evt, err = q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "btn1" {
		t.Errorf("source = %q, want btn1", evt.Source)
	}
}

func TestEventQueueFilterNoMatch(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{Type: "click", Source: "btn1"})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Filter for something that doesn't exist.
	opts := &DequeueOpts{Filter: []string{"btn99"}}
	evt, err := q.Dequeue(ctx, opts)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if evt != nil {
		t.Error("expected nil event")
	}

	// The btn1 event should still be there.
	ctx2 := context.Background()
	evt, err = q.Dequeue(ctx2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "btn1" {
		t.Errorf("source = %q, want btn1", evt.Source)
	}
}

func TestEventQueueDebounce(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	// Enqueue 5 rapid events from the same source.
	for i := 0; i < 5; i++ {
		q.Enqueue(&Event{Type: "change", Source: "input1", Data: map[string]any{"i": i}})
	}

	ctx := context.Background()
	opts := &DequeueOpts{DebounceMs: 50}

	evt, err := q.Dequeue(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	// Should get the last event.
	if evt.Source != "input1" {
		t.Errorf("source = %q", evt.Source)
	}
	if evt.CoalescedCount == 0 {
		t.Error("expected coalesced_count > 0")
	}
}

func TestEventQueueConcurrentEnqueue(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	var wg sync.WaitGroup
	n := 100
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Enqueue(&Event{Type: "test", Source: "concurrent"})
		}()
	}
	wg.Wait()

	// Should be able to dequeue all n events.
	ctx := context.Background()
	for i := 0; i < n; i++ {
		evt, err := q.Dequeue(ctx, nil)
		if err != nil {
			t.Fatalf("dequeue %d: %v", i, err)
		}
		if evt.Source != "concurrent" {
			t.Errorf("event %d source = %q", i, evt.Source)
		}
	}
}

func TestEventQueueClose(t *testing.T) {
	q := NewEventQueue()

	done := make(chan error, 1)
	go func() {
		ctx := context.Background()
		_, err := q.Dequeue(ctx, nil)
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	q.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error from dequeue on closed queue")
		}
	case <-time.After(time.Second):
		t.Fatal("dequeue didn't return after close")
	}
}

func TestEventQueueContextCancel(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := q.Dequeue(ctx, nil)
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error from cancelled context")
		}
	case <-time.After(time.Second):
		t.Fatal("dequeue didn't return after cancel")
	}
}

func TestEventWithContext(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{
		Type:   "click",
		Source: "btn1",
		Context: map[string]map[string]any{
			"input1": {"value": "hello"},
			"input2": {"value": "world"},
		},
	})

	ctx := context.Background()
	evt, err := q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Context["input1"]["value"] != "hello" {
		t.Errorf("context input1 = %v", evt.Context["input1"])
	}
}

func TestEventWithDOMSummary(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{
		Type:       "click",
		Source:     "btn1",
		DOMSummary: "root(header, main(form, table))",
	})

	ctx := context.Background()
	evt, err := q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.DOMSummary != "root(header, main(form, table))" {
		t.Errorf("dom_summary = %q", evt.DOMSummary)
	}
}

func TestEventQueueEnqueueAfterClose(_ *testing.T) {
	q := NewEventQueue()
	q.Close()

	// Should not panic.
	q.Enqueue(&Event{Type: "click", Source: "btn1"})
}

func TestEventQueueDebounceRealtime(t *testing.T) {
	// Test events arriving DURING the debounce window.
	q := NewEventQueue()
	defer q.Close()

	// Enqueue first event to start the debounce.
	q.Enqueue(&Event{Type: "change", Source: "input1", Data: map[string]any{"v": "a"}})

	done := make(chan *Event, 1)
	go func() {
		ctx := context.Background()
		evt, _ := q.Dequeue(ctx, &DequeueOpts{DebounceMs: 100})
		done <- evt
	}()

	// Send more events during the debounce window.
	time.Sleep(30 * time.Millisecond)
	q.Enqueue(&Event{Type: "change", Source: "input1", Data: map[string]any{"v": "b"}})
	time.Sleep(30 * time.Millisecond)
	q.Enqueue(&Event{Type: "change", Source: "input1", Data: map[string]any{"v": "c"}})

	select {
	case evt := <-done:
		if evt == nil {
			t.Fatal("got nil event")
		}
		// Should get the last event with coalesced count.
		if evt.Data["v"] != "c" {
			t.Errorf("expected last value 'c', got %v", evt.Data["v"])
		}
		if evt.CoalescedCount < 2 {
			t.Errorf("coalesced_count = %d, want >= 2", evt.CoalescedCount)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("debounce didn't complete")
	}
}

func TestEventQueueFilterMultipleSources(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{Type: "click", Source: "btn1"})
	q.Enqueue(&Event{Type: "click", Source: "btn2"})
	q.Enqueue(&Event{Type: "click", Source: "btn3"})

	ctx := context.Background()
	opts := &DequeueOpts{Filter: []string{"btn2", "btn3"}}

	// Should get btn2 first (FIFO among matches).
	evt, err := q.Dequeue(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "btn2" {
		t.Errorf("source = %q, want btn2", evt.Source)
	}

	// Then btn3.
	evt, err = q.Dequeue(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "btn3" {
		t.Errorf("source = %q, want btn3", evt.Source)
	}

	// btn1 should remain.
	evt, err = q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Source != "btn1" {
		t.Errorf("remaining source = %q, want btn1", evt.Source)
	}
}

func TestEventQueueDoubleClose(_ *testing.T) {
	q := NewEventQueue()
	q.Close()
	// Second close should not panic.
	q.Close()
}

// TestEventContextAutoCollection documents that context and dom_summary
// are caller-populated fields. Auto-collection from the tree is deferred
// to the render layer (M5), which has access to the live tree state.
// The event queue itself is tree-agnostic by design.
func TestEventContextAutoCollection(t *testing.T) {
	// Build a tree and manually collect context, as the render layer will do.
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("value", "hello")
	tree.Find("a2").SetProp("value", "world")

	// Simulate what the render layer will do: collect sibling context.
	source := tree.Find("b1")
	parent := source.Parent()
	eventCtx := make(map[string]map[string]any)
	for _, sibling := range parent.Children {
		if sibling.ID != source.ID && len(sibling.Props) > 0 {
			eventCtx[sibling.ID] = copyMap(sibling.Props)
		}
	}

	q := NewEventQueue()
	defer q.Close()
	q.Enqueue(&Event{
		Type:       "click",
		Source:     "b1",
		Context:    eventCtx,
		DOMSummary: tree.Summary(),
	})

	ctx := context.Background()
	evt, err := q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.DOMSummary != "root(a(a1, a2), b(b1))" {
		t.Errorf("dom_summary = %q", evt.DOMSummary)
	}
	// In this case b1's parent is b, and b has no other children with props,
	// so context should be empty. This documents the expected pattern.
	if len(evt.Context) != 0 {
		t.Errorf("expected empty context for b1 (no siblings with props), got %v", evt.Context)
	}
}

func TestEventDataField(t *testing.T) {
	q := NewEventQueue()
	defer q.Close()

	q.Enqueue(&Event{
		Type:   "change",
		Source: "input1",
		Data:   map[string]any{"value": "typed text", "cursor": 10},
	})

	ctx := context.Background()
	evt, err := q.Dequeue(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Data["value"] != "typed text" {
		t.Errorf("data value = %v", evt.Data["value"])
	}
	if evt.Data["cursor"] != 10 {
		t.Errorf("data cursor = %v", evt.Data["cursor"])
	}
}
