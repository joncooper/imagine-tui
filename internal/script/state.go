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

// stateProxy exposes top-level state reads/writes while keeping the backing
// object persistent across script invocations.
type stateProxy struct {
	rt     *Runtime
	nodeID string
}

func (p *stateProxy) Get(key string) goja.Value {
	p.rt.recordSourceDep(stateSource(p.nodeID, key))

	state := p.rt.getOrCreateState(p.nodeID)
	val := state.Get(key)
	if val == nil {
		return goja.Undefined()
	}
	return val
}

func (p *stateProxy) Set(key string, val goja.Value) bool {
	state := p.rt.getOrCreateState(p.nodeID)
	if err := state.Set(key, val); err != nil {
		return false
	}
	p.rt.markDirtyState(p.nodeID, key)
	return true
}

func (p *stateProxy) Has(key string) bool {
	p.rt.recordSourceDep(stateSource(p.nodeID, key))

	state := p.rt.getOrCreateState(p.nodeID)
	val := state.Get(key)
	return val != nil && !goja.IsUndefined(val)
}

func (p *stateProxy) Delete(key string) bool {
	state := p.rt.getOrCreateState(p.nodeID)
	if err := state.Delete(key); err != nil {
		return false
	}
	p.rt.markDirtyState(p.nodeID, key)
	return true
}

func (p *stateProxy) Keys() []string {
	state := p.rt.getOrCreateState(p.nodeID)
	keys := state.Keys()
	if keys == nil {
		return []string{}
	}
	return keys
}

// makeStateProxy returns a JS object that proxies top-level state access for a node.
// Must be called with rt.mu held.
func (rt *Runtime) makeStateProxy(nodeID string) goja.Value {
	rt.getOrCreateState(nodeID)
	return rt.vm.NewDynamicObject(&stateProxy{rt: rt, nodeID: nodeID})
}

// removeState deletes the per-node state for the given node ID.
// Called when a node is removed from the DOM.
func (rt *Runtime) removeState(nodeID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.states, nodeID)
}
