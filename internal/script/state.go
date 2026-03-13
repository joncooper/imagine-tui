package script

import "github.com/dop251/goja"

// getOrCreateState returns the per-node state object, creating one if needed.
// Must be called with rt.mu held.
func (rt *Runtime) getOrCreateState(nodeID string) *goja.Object {
	if s, ok := rt.states[nodeID]; ok {
		return s
	}
	s := rt.vm.NewObject()
	rt.states[nodeID] = s
	return s
}

// removeState deletes the per-node state for the given node ID.
// Called when a node is removed from the DOM.
func (rt *Runtime) removeState(nodeID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.states, nodeID)
}
