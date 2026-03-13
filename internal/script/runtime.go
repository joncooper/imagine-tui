package script

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// Default script execution timeout.
const defaultTimeout = 10 * time.Millisecond

// Runtime is the script execution engine for a session.
// It owns a single goja VM and manages per-node state.
type Runtime struct {
	mu       sync.Mutex
	vm       *goja.Runtime
	tree     *dom.Tree
	events   *dom.EventQueue
	states   map[string]*goja.Object // nodeID -> persistent JS state
	dirty    map[string]bool         // nodes modified since last propagation
	timeout  time.Duration
	debugLog func(string)
}

// Option configures the Runtime.
type Option func(*Runtime)

// WithTimeout sets the per-script execution timeout.
func WithTimeout(d time.Duration) Option {
	return func(rt *Runtime) {
		rt.timeout = d
	}
}

// WithDebugLog sets a function to receive debug() output from scripts.
func WithDebugLog(fn func(string)) Option {
	return func(rt *Runtime) {
		rt.debugLog = fn
	}
}

// New creates a new script Runtime bound to a DOM tree and event queue.
func New(tree *dom.Tree, events *dom.EventQueue, opts ...Option) *Runtime {
	rt := &Runtime{
		vm:      goja.New(),
		tree:    tree,
		events:  events,
		states:  make(map[string]*goja.Object),
		dirty:   make(map[string]bool),
		timeout: defaultTimeout,
	}
	for _, opt := range opts {
		opt(rt)
	}
	rt.initSandbox()
	return rt
}

// initSandbox strips dangerous globals and provides safe replacements.
func (rt *Runtime) initSandbox() {
	// Delete dangerous globals by setting them to undefined.
	dangerous := []string{
		"require", "importScripts",
		"setTimeout", "setInterval", "setImmediate",
		"clearTimeout", "clearInterval", "clearImmediate",
		"fetch", "XMLHttpRequest",
		"process", "globalThis",
	}
	global := rt.vm.GlobalObject()
	for _, name := range dangerous {
		_ = global.Delete(name)
	}

	// Neuter console.
	console := rt.vm.NewObject()
	_ = console.Set("log", goja.Undefined())
	_ = console.Set("warn", goja.Undefined())
	_ = console.Set("error", goja.Undefined())
	_ = rt.vm.Set("console", console)

	// Provide debug() that routes to server log.
	_ = rt.vm.Set("debug", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = a.String()
		}
		if rt.debugLog != nil {
			rt.debugLog(strings.Join(args, " "))
		}
		return goja.Undefined()
	})
}

// execScript runs a script body in the context of a specific node.
// The caller must NOT hold rt.mu.
func (rt *Runtime) execScript(nodeID, hook, body string, payload *HookPayload) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	node := rt.tree.Find(nodeID)
	if node == nil {
		return &Error{NodeID: nodeID, Hook: hook, Message: "node not found"}
	}

	rt.setupContext(node, payload)

	timer := time.AfterFunc(rt.timeout, func() {
		rt.vm.Interrupt("script timeout")
	})

	_, err := rt.vm.RunString(body)
	timer.Stop()
	rt.vm.ClearInterrupt()

	if err != nil {
		var ie *goja.InterruptedError
		if errors.As(err, &ie) {
			return &Error{NodeID: nodeID, Hook: hook, Message: ie.String(), IsTimeout: true}
		}
		return &Error{NodeID: nodeID, Hook: hook, Message: err.Error()}
	}

	return nil
}

// setupContext binds $, state, emit, and event globals for a script invocation.
// Must be called with rt.mu held.
func (rt *Runtime) setupContext(node *dom.Node, payload *HookPayload) {
	// Minimal setup for sandbox phase. Full $ proxy implemented in later steps.
	_ = node
	_ = payload
}

// HookPayload is the data passed to a hook script.
type HookPayload struct {
	Event  string         `json:"event,omitempty"`
	Source string         `json:"source,omitempty"`
	Key    string         `json:"key,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}
