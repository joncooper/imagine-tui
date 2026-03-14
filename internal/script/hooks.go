package script

// HookType enumerates the lifecycle hooks.
type HookType string

// Lifecycle hook types.
const (
	HookOnMount  HookType = "on_mount"
	HookOnChange HookType = "on_change"
	HookOnSubmit HookType = "on_submit"
	HookOnEvent  HookType = "on_event"
	HookOnFocus  HookType = "on_focus"
	HookOnBlur   HookType = "on_blur"
	HookOnKey    HookType = "on_key"
)

// ExecHook runs a lifecycle hook script for the given node.
// Returns the set of dirty node IDs (nodes whose props were modified).
// If the node has no script for the given hook, this is a no-op.
func (rt *Runtime) ExecHook(nodeID string, hook HookType, payload *HookPayload) ([]string, error) {
	return rt.execHook(nodeID, hook, payload, true)
}

func (rt *Runtime) execHook(nodeID string, hook HookType, payload *HookPayload, resetDirty bool) ([]string, error) {
	node := rt.tree.Find(nodeID)
	if node == nil {
		return nil, &Error{NodeID: nodeID, Hook: string(hook), Message: "node not found"}
	}

	body, ok := node.Scripts[string(hook)]
	if !ok || body == "" {
		return nil, nil
	}

	// Clear dirty set before execution unless the caller is carrying forward an
	// already-dirty node (for example NotifyChange setting $.value first).
	rt.mu.Lock()
	if resetDirty {
		rt.dirty = make(map[string]bool)
		rt.dirtySources = make(map[sourceKey]bool)
	}
	rt.mu.Unlock()

	err := rt.execScript(nodeID, string(hook), body, payload)
	if err != nil {
		return nil, err
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	changed := make(map[string]bool)
	if err := rt.propagateChangesLocked(changed); err != nil {
		return nil, err
	}

	dirty := make([]string, 0, len(changed))
	for id := range changed {
		dirty = append(dirty, id)
	}
	return dirty, nil
}

// NotifyMount should be called when a node is inserted into the tree.
// Fires the on_mount hook if present.
func (rt *Runtime) NotifyMount(nodeID string) error {
	_, err := rt.ExecHook(nodeID, HookOnMount, nil)
	return err
}

// NotifyRemove should be called when a node is removed from the tree.
// Cleans up per-node state and dependency graph entries.
func (rt *Runtime) NotifyRemove(nodeID string) {
	rt.removeState(nodeID)
	rt.mu.Lock()
	rt.deps.RemoveNode(nodeID)
	rt.mu.Unlock()
}

// NotifyChange should be called when a node's value changes.
// Sets the value prop, then fires the on_change hook if present.
func (rt *Runtime) NotifyChange(nodeID string, newValue any) error {
	node := rt.tree.Find(nodeID)
	if node == nil {
		return &Error{NodeID: nodeID, Hook: "on_change", Message: "node not found"}
	}

	node.SetProp("value", newValue)
	rt.mu.Lock()
	rt.markDirtyProp(nodeID, "value")
	rt.mu.Unlock()

	if body, ok := node.Scripts[string(HookOnChange)]; ok && body != "" {
		_, err := rt.execHook(nodeID, HookOnChange, &HookPayload{
			Data: map[string]any{"value": newValue},
		}, false)
		return err
	}

	return rt.PropagateChanges()
}
