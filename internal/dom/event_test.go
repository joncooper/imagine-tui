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
