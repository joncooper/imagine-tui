package dom

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event represents a user interaction event queued for Claude Code.
type Event struct {
	Type           string                    `json:"event"`
	Source         string                    `json:"source"`
	Data           map[string]any            `json:"data,omitempty"`
	Context        map[string]map[string]any `json:"context,omitempty"`
	DOMSummary     string                    `json:"dom_summary,omitempty"`
	CoalescedCount int                       `json:"coalesced_count,omitempty"`
}

// DequeueOpts configures the behavior of Dequeue.
type DequeueOpts struct {
	Filter     []string // only accept events from these source node IDs
	DebounceMs int      // coalesce rapid events within this window
}

// EventQueue is a thread-safe FIFO queue for Claude-routed events.
type EventQueue struct {
	mu     sync.Mutex
	events []*Event
	notify chan struct{}
	closed bool
}

// NewEventQueue creates a new event queue.
func NewEventQueue() *EventQueue {
	return &EventQueue{
		notify: make(chan struct{}, 1),
	}
}

// Enqueue adds an event to the queue. Thread-safe. Does nothing if closed.
func (q *EventQueue) Enqueue(evt *Event) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.events = append(q.events, evt)

	// Signal any waiting Dequeue.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Dequeue blocks until an event is available, the context is canceled, or the
// queue is closed. Returns the event or an error.
//
// If opts.Filter is set, only events from those source IDs are returned.
// Non-matching events remain in the queue.
//
// If opts.DebounceMs > 0, after finding a matching event the queue waits for
// the debounce window, coalescing additional events from the same source.
func (q *EventQueue) Dequeue(ctx context.Context, opts *DequeueOpts) (*Event, error) {
	filter := makeFilterSet(opts)
	debounce := 0
	if opts != nil {
		debounce = opts.DebounceMs
	}

	for {
		// Try to take a matching event.
		evt := q.take(filter)
		if evt != nil {
			if debounce > 0 {
				return q.debounceWait(ctx, evt, filter, debounce)
			}
			return evt, nil
		}

		// Wait for notification, context cancellation, or close.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dequeue: %w", ctx.Err())
		case _, ok := <-q.notify:
			if !ok {
				return nil, fmt.Errorf("dequeue: queue closed")
			}
			// Loop around and try again.
		}
	}
}

// Close shuts down the queue. Any blocked Dequeue calls will return an error.
func (q *EventQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.notify)
	}
}

// take removes and returns the first event matching the filter.
// Returns nil if no matching event is available.
func (q *EventQueue) take(filter map[string]bool) *Event {
	q.mu.Lock()
	defer q.mu.Unlock()

	if filter == nil {
		if len(q.events) == 0 {
			return nil
		}
		evt := q.events[0]
		q.events = q.events[1:]
		return evt
	}

	for i, evt := range q.events {
		if filter[evt.Source] {
			q.events = append(q.events[:i], q.events[i+1:]...)
			return evt
		}
	}
	return nil
}

// debounceWait waits for the debounce window and coalesces events.
func (q *EventQueue) debounceWait(ctx context.Context, first *Event, filter map[string]bool, debounceMs int) (*Event, error) {
	timer := time.NewTimer(time.Duration(debounceMs) * time.Millisecond)
	defer timer.Stop()

	latest := first
	coalesced := 0

	for {
		select {
		case <-timer.C:
			latest.CoalescedCount = coalesced
			return latest, nil
		case <-ctx.Done():
			latest.CoalescedCount = coalesced
			return latest, nil
		case _, ok := <-q.notify:
			if !ok {
				latest.CoalescedCount = coalesced
				return latest, nil
			}
			// Check for newer matching events.
			for {
				evt := q.take(filter)
				if evt == nil {
					break
				}
				latest = evt
				coalesced++
			}
			// Reset the debounce timer.
			timer.Reset(time.Duration(debounceMs) * time.Millisecond)
		}
	}
}

func makeFilterSet(opts *DequeueOpts) map[string]bool {
	if opts == nil || len(opts.Filter) == 0 {
		return nil
	}
	set := make(map[string]bool, len(opts.Filter))
	for _, id := range opts.Filter {
		set[id] = true
	}
	return set
}
