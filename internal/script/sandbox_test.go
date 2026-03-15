package script

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// helper to create a minimal runtime for sandbox tests.
func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	events := dom.NewEventQueue()
	rt := New(tree, events)
	return rt
}

func TestForbiddenAPIs(t *testing.T) {
	rt := newTestRuntime(t)

	forbidden := []struct {
		name   string
		script string
	}{
		{"require", `require('fs')`},
		{"fetch", `fetch('http://example.com')`},
		{"setImmediate", `setImmediate(function(){})`},
		{"process", `process.exit(1)`},
	}

	for _, tc := range forbidden {
		t.Run(tc.name, func(t *testing.T) {
			err := rt.execScript("root", "test", tc.script, nil)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
			var se *Error
			if !errors.As(err, &se) {
				t.Errorf("expected script.Error, got %T: %v", err, err)
			}
		})
	}
}

func TestTimerGlobalsAvailable(t *testing.T) {
	rt := newTestRuntime(t)

	err := rt.execScript("root", "test", `
		if (typeof setTimeout !== "function") {
			throw new Error("expected setTimeout to be available");
		}
		if (typeof setInterval !== "function") {
			throw new Error("expected setInterval to be available");
		}
		if (typeof clearTimeout !== "function") {
			throw new Error("expected clearTimeout to be available");
		}
		if (typeof clearInterval !== "function") {
			throw new Error("expected clearInterval to be available");
		}
	`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyntaxError(t *testing.T) {
	rt := newTestRuntime(t)

	err := rt.execScript("root", "test", `function(`, nil)
	if err == nil {
		t.Fatal("expected error for syntax error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if se.NodeID != "root" {
		t.Errorf("expected NodeID 'root', got %q", se.NodeID)
	}
	if se.Hook != "test" {
		t.Errorf("expected Hook 'test', got %q", se.Hook)
	}
	if se.IsTimeout {
		t.Error("expected IsTimeout to be false")
	}
}

func TestRuntimeError(t *testing.T) {
	rt := newTestRuntime(t)

	err := rt.execScript("root", "test", `null.foo`, nil)
	if err == nil {
		t.Fatal("expected error for runtime error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if se.IsTimeout {
		t.Error("expected IsTimeout to be false")
	}
}

func TestInfiniteLoopTimeout(t *testing.T) {
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

	start := time.Now()
	err = rt.execScript("root", "test", `while(true){}`, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for infinite loop")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if !se.IsTimeout {
		t.Error("expected IsTimeout to be true")
	}
	// Should complete within a reasonable margin of the timeout.
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestDebugRoutesToLogSink(t *testing.T) {
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	events := dom.NewEventQueue()

	var logged []string
	rt := New(tree, events, WithDebugLog(func(msg string) {
		logged = append(logged, msg)
	}))

	err = rt.execScript("root", "test", `debug("hello", "world")`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logged) != 1 {
		t.Fatalf("expected 1 log message, got %d", len(logged))
	}
	if !strings.Contains(logged[0], "hello") || !strings.Contains(logged[0], "world") {
		t.Errorf("expected log to contain 'hello' and 'world', got %q", logged[0])
	}
}

func TestSuccessfulScript(t *testing.T) {
	rt := newTestRuntime(t)

	// A simple script that doesn't error should return nil.
	err := rt.execScript("root", "test", `var x = 1 + 2;`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecScriptNodeNotFound(t *testing.T) {
	rt := newTestRuntime(t)

	err := rt.execScript("nonexistent", "test", `var x = 1;`, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected script.Error, got %T: %v", err, err)
	}
	if se.NodeID != "nonexistent" {
		t.Errorf("expected NodeID 'nonexistent', got %q", se.NodeID)
	}
}
