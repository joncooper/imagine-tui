package script

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSetTimeoutFiresWhenDue(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	if err := rt.execScript("child1", "test", `
		setTimeout(function() {
			$.text = "later";
		}, 10)
	`, nil); err != nil {
		t.Fatalf("execScript: %v", err)
	}

	if ran, err := rt.RunDueTimers(time.Now()); err != nil {
		t.Fatalf("RunDueTimers before due: %v", err)
	} else if ran {
		t.Fatal("expected no timer callback before due")
	}

	if ran, err := rt.RunDueTimers(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatalf("RunDueTimers when due: %v", err)
	} else if !ran {
		t.Fatal("expected timer callback to run")
	}

	node := rt.tree.Find("child1")
	if node == nil {
		t.Fatal("expected child1 node")
	}
	if got, _ := node.GetProp("text"); got != "later" {
		t.Fatalf("expected text to be updated by timeout, got %v", got)
	}
}

func TestClearTimeoutCancelsPendingTimer(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	if err := rt.execScript("child1", "test", `
		var id = setTimeout(function() {
			$.text = "should-not-run";
		}, 10);
		clearTimeout(id);
	`, nil); err != nil {
		t.Fatalf("execScript: %v", err)
	}

	if ran, err := rt.RunDueTimers(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatalf("RunDueTimers: %v", err)
	} else if ran {
		t.Fatal("expected cleared timeout not to run")
	}

	node := rt.tree.Find("child1")
	if node == nil {
		t.Fatal("expected child1 node")
	}
	if got, _ := node.GetProp("text"); got != "hello" {
		t.Fatalf("expected text to remain unchanged, got %v", got)
	}
}

func TestSetIntervalFiresUntilCleared(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	if err := rt.execScript("child1", "test", `
		state.count = 0;
		var id = setInterval(function() {
			state.count += 1;
			$.text = "tick:" + state.count;
			if (state.count === 2) {
				clearInterval(id);
			}
		}, 10);
	`, nil); err != nil {
		t.Fatalf("execScript: %v", err)
	}

	if ran, err := rt.RunDueTimers(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatalf("first RunDueTimers: %v", err)
	} else if !ran {
		t.Fatal("expected first interval callback to run")
	}

	node := rt.tree.Find("child1")
	if node == nil {
		t.Fatal("expected child1 node")
	}
	if got, _ := node.GetProp("text"); got != "tick:1" {
		t.Fatalf("expected first interval update, got %v", got)
	}

	if ran, err := rt.RunDueTimers(time.Now().Add(40 * time.Millisecond)); err != nil {
		t.Fatalf("second RunDueTimers: %v", err)
	} else if !ran {
		t.Fatal("expected second interval callback to run")
	}

	if got, _ := node.GetProp("text"); got != "tick:2" {
		t.Fatalf("expected second interval update, got %v", got)
	}

	if ran, err := rt.RunDueTimers(time.Now().Add(60 * time.Millisecond)); err != nil {
		t.Fatalf("third RunDueTimers: %v", err)
	} else if ran {
		t.Fatal("expected cleared interval not to run again")
	}
}

func TestTimersCancelOnNotifyRemove(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	if err := rt.execScript("child1", "test", `
		setTimeout(function() {
			$.text = "should-not-run";
		}, 10)
	`, nil); err != nil {
		t.Fatalf("execScript: %v", err)
	}

	rt.NotifyRemove("child1")

	if ran, err := rt.RunDueTimers(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatalf("RunDueTimers: %v", err)
	} else if ran {
		t.Fatal("expected removed node timers not to run")
	}
}

func TestSetTimeoutRejectsOverMaxDuration(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	rt = New(rt.tree, rt.events, WithTimerLimits(25*time.Millisecond, 4))

	err := rt.execScript("child1", "test", `setTimeout(function(){}, 50)`, nil)
	if err == nil {
		t.Fatal("expected timer duration error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message, "max timer duration") {
		t.Fatalf("expected max timer duration message, got %q", se.Message)
	}
}

func TestSetTimeoutRejectsWhenMaxConcurrentTimersExceeded(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	rt = New(rt.tree, rt.events, WithTimerLimits(100*time.Millisecond, 1))

	err := rt.execScript("child1", "test", `
		setTimeout(function() {}, 10);
		setTimeout(function() {}, 10);
	`, nil)
	if err == nil {
		t.Fatal("expected max concurrent timer error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message, "max concurrent timers") {
		t.Fatalf("expected max concurrent timers message, got %q", se.Message)
	}

	if len(rt.timers) != 1 {
		t.Fatalf("expected first timer to remain scheduled, got %d timers", len(rt.timers))
	}
}
