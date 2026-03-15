package script

import (
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// DepGraph tracks computed property dependencies.
type DepGraph struct {
	forward map[depKey]map[sourceKey]bool // computed prop -> set of sources it reads
	reverse map[sourceKey][]depKey        // source -> computed props that depend on it
}

type depKey struct {
	NodeID   string
	PropName string
}

type sourceKey struct {
	NodeID string
	Key    string
}

func propSource(nodeID, propName string) sourceKey {
	return sourceKey{NodeID: nodeID, Key: "prop:" + propName}
}

func stateSource(nodeID, key string) sourceKey {
	return sourceKey{NodeID: nodeID, Key: "state:" + key}
}

// newDepGraph creates an empty dependency graph.
func newDepGraph() *DepGraph {
	return &DepGraph{
		forward: make(map[depKey]map[sourceKey]bool),
		reverse: make(map[sourceKey][]depKey),
	}
}

// Update sets the dependencies for a computed prop, updating both forward and reverse maps.
func (dg *DepGraph) Update(key depKey, accessed map[sourceKey]bool) {
	// Remove old reverse entries.
	if old, ok := dg.forward[key]; ok {
		for oldDep := range old {
			dg.removeReverse(oldDep, key)
		}
	}
	// Set new forward mapping.
	dg.forward[key] = accessed
	// Set new reverse entries.
	for dep := range accessed {
		dg.reverse[dep] = append(dg.reverse[dep], key)
	}
}

// Dependents returns all computed props that depend on the given nodeID.
func (dg *DepGraph) Dependents(nodeID string) []depKey {
	set := make(map[depKey]bool)
	for source, deps := range dg.reverse {
		if source.NodeID != nodeID {
			continue
		}
		for _, dk := range deps {
			set[dk] = true
		}
	}
	result := make([]depKey, 0, len(set))
	for dk := range set {
		result = append(result, dk)
	}
	return result
}

func (dg *DepGraph) dependentsForSource(source sourceKey) []depKey {
	return dg.reverse[source]
}

// RemoveNode removes all entries for a node (both as a dependency and as a computed prop owner).
func (dg *DepGraph) RemoveNode(nodeID string) {
	// Remove all computed props owned by this node.
	for key := range dg.forward {
		if key.NodeID == nodeID {
			for dep := range dg.forward[key] {
				dg.removeReverse(dep, key)
			}
			delete(dg.forward, key)
		}
	}
	// Remove as a dependency source.
	for source := range dg.reverse {
		if source.NodeID == nodeID {
			delete(dg.reverse, source)
		}
	}
}

// DetectCycle checks if evaluating the given computed prop would eventually
// trigger itself to be re-evaluated via the dirty propagation chain:
// evaluating dk dirties dk.NodeID → computed props depending on dk.NodeID
// get re-evaluated → dirtying their nodes → etc. Returns true if a cycle exists.
func (dg *DepGraph) DetectCycle(startKey depKey) bool {
	visited := make(map[sourceKey]bool)
	return dg.wouldTrigger(propSource(startKey.NodeID, startKey.PropName), startKey, visited)
}

// wouldTrigger checks if dirtying dirtyNodeID would eventually cause target
// to need re-evaluation.
func (dg *DepGraph) wouldTrigger(dirtySource sourceKey, target depKey, visited map[sourceKey]bool) bool {
	if visited[dirtySource] {
		return false
	}
	visited[dirtySource] = true

	for _, dk := range dg.reverse[dirtySource] {
		if dk == target {
			return true
		}
		// Evaluating dk would dirty dk.NodeID.
		if dg.wouldTrigger(propSource(dk.NodeID, dk.PropName), target, visited) {
			return true
		}
	}
	return false
}

func (dg *DepGraph) removeReverse(source sourceKey, key depKey) {
	deps := dg.reverse[source]
	for i, dk := range deps {
		if dk == key {
			dg.reverse[source] = append(deps[:i], deps[i+1:]...)
			break
		}
	}
	if len(dg.reverse[source]) == 0 {
		delete(dg.reverse, source)
	}
}

// evalCtx tracks state during a single computed prop evaluation.
type evalCtx struct {
	ownerNodeID string
	propName    string
	accessed    map[sourceKey]bool
}

// EvalComputed evaluates a computed prop expression and returns the result.
// It tracks which nodes the expression reads via $('id') for dependency tracking.
func (rt *Runtime) EvalComputed(nodeID, propName, expr string) (any, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	node := rt.tree.Find(nodeID)
	if node == nil {
		return nil, &Error{NodeID: nodeID, Hook: propName, Message: "node not found"}
	}

	return rt.evalComputedLocked(nodeID, propName, expr)
}

// evalComputedLocked is the internal version that assumes rt.mu is held.
func (rt *Runtime) evalComputedLocked(nodeID, propName, expr string) (any, error) {
	node := rt.tree.Find(nodeID)
	if node == nil {
		return nil, &Error{NodeID: nodeID, Hook: propName, Message: "node not found"}
	}

	// Set up tracking context.
	rt.currentEval = &evalCtx{
		ownerNodeID: nodeID,
		propName:    propName,
		accessed:    make(map[sourceKey]bool),
	}
	defer func() { rt.currentEval = nil }()

	rt.setupContext(node, nil)

	// Wrap the expression in a function body so 'return' works.
	wrapped := "(function(){ " + expr + " })()"

	timer := rt.startTimeout()
	val, err := rt.vm.RunString(wrapped)
	rt.stopTimeout(timer)

	if err != nil {
		var ie *goja.InterruptedError
		if errors.As(err, &ie) {
			return nil, &Error{NodeID: nodeID, Hook: propName, Message: ie.String(), IsTimeout: true}
		}
		return nil, &Error{NodeID: nodeID, Hook: propName, Message: err.Error()}
	}

	// Update dependency graph.
	rt.deps.Update(depKey{nodeID, propName}, rt.currentEval.accessed)

	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil, nil
	}
	return val.Export(), nil
}

// PropagateChanges re-evaluates computed props that depend on dirty nodes.
// Iterates until no new dirty nodes are produced, with a cap to prevent infinite loops.
func (rt *Runtime) PropagateChanges() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	return rt.propagateChangesLocked(nil)
}

// propagateChangesLocked re-evaluates computed props that depend on dirty nodes.
// Must be called with rt.mu held. If changed is non-nil, all directly dirty and
// recomputed node IDs are recorded into it.
func (rt *Runtime) propagateChangesLocked(changed map[string]bool) error {
	const maxIterations = 100
	for i := 0; i < maxIterations; i++ {
		if len(rt.dirty) == 0 && len(rt.dirtySources) == 0 {
			return nil
		}

		// Snapshot and clear current dirty set.
		batch := rt.dirty
		rt.dirty = make(map[string]bool)
		sourceBatch := rt.dirtySources
		rt.dirtySources = make(map[sourceKey]bool)

		pending := make(map[depKey]bool)

		if len(sourceBatch) == 0 {
			for nodeID := range batch {
				if changed != nil {
					changed[nodeID] = true
				}
				for _, dk := range rt.deps.Dependents(nodeID) {
					pending[dk] = true
				}
			}
		}

		for source := range sourceBatch {
			if changed != nil {
				changed[source.NodeID] = true
			}
			for _, dk := range rt.deps.dependentsForSource(source) {
				pending[dk] = true
			}
		}

		for dk := range pending {
			// Cycle detection.
			if rt.deps.DetectCycle(dk) {
				return &Error{
					NodeID:  dk.NodeID,
					Hook:    dk.PropName,
					Message: fmt.Sprintf("cycle detected in computed prop: %s.%s", dk.NodeID, dk.PropName),
				}
			}

			node := rt.tree.Find(dk.NodeID)
			if node == nil {
				continue
			}
			expr, ok := node.Computed[dk.PropName]
			if !ok {
				continue
			}

			val, err := rt.evalComputedLocked(dk.NodeID, dk.PropName, expr)
			if err != nil {
				return err
			}
			node.SetProp(dk.PropName, val)
			rt.markDirtyProp(dk.NodeID, dk.PropName)
			if changed != nil {
				changed[dk.NodeID] = true
			}
		}
	}

	return &Error{
		NodeID:  "",
		Hook:    "propagation",
		Message: "computed prop propagation exceeded iteration limit",
	}
}

// EvalAllComputed evaluates all computed props in the tree.
func (rt *Runtime) EvalAllComputed() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	return rt.evalAllComputedWalk(rt.tree.Root)
}

// evalAllComputedWalk recursively evaluates computed props. Must be called with rt.mu held.
func (rt *Runtime) evalAllComputedWalk(node *dom.Node) error {
	for propName, expr := range node.Computed {
		val, err := rt.evalComputedLocked(node.ID, propName, expr)
		if err != nil {
			return err
		}
		node.SetProp(propName, val)
	}
	for _, child := range node.Children {
		if err := rt.evalAllComputedWalk(child); err != nil {
			return err
		}
	}
	return nil
}
