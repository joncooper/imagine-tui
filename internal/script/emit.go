package script

import (
	"encoding/json"
	"fmt"

	"github.com/dop251/goja"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// makeEmitFn returns a Go function exposed to JS as emit(target, data).
// Must be called with rt.mu held (it is bound during setupContext).
func (rt *Runtime) makeEmitFn(sourceNodeID string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(rt.vm.NewTypeError("emit requires two arguments: target and data"))
		}

		target := call.Arguments[0].String()

		switch target {
		case "local":
			rt.emitLocal(call.Arguments[1])
		case "agent", "claude":
			rt.emitAgent(sourceNodeID, call.Arguments[1])
		default:
			panic(rt.vm.NewTypeError(fmt.Sprintf("emit: invalid target %q (expected 'local', 'agent', or deprecated 'claude')", target)))
		}

		return goja.Undefined()
	}
}

// emitLocal applies patch operations to the DOM synchronously.
func (rt *Runtime) emitLocal(opsVal goja.Value) {
	exported := opsVal.Export()
	opsJSON, err := json.Marshal(exported)
	if err != nil {
		panic(rt.vm.NewTypeError(fmt.Sprintf("emit local: invalid ops: %v", err)))
	}
	ops, err := dom.ParsePatchOps(opsJSON)
	if err != nil {
		panic(rt.vm.NewTypeError(fmt.Sprintf("emit local: %v", err)))
	}
	if err := rt.tree.Patch(ops); err != nil {
		panic(rt.vm.NewTypeError(fmt.Sprintf("emit local: %v", err)))
	}
}

// emitAgent enqueues an event for the MCP client via the event queue.
func (rt *Runtime) emitAgent(sourceNodeID string, dataVal goja.Value) {
	var data map[string]any
	if exported, ok := dataVal.Export().(map[string]any); ok {
		data = exported
	}
	rt.events.Enqueue(&dom.Event{
		Type:   "script",
		Source: sourceNodeID,
		Data:   data,
	})
}
