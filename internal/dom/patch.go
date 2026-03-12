package dom

import (
	"encoding/json"
	"fmt"
)

// OpType is the type of a patch operation.
type OpType string

// Patch operation types.
const (
	OpUpdate OpType = "update"
	OpInsert OpType = "insert"
	OpRemove OpType = "remove"
	OpMove   OpType = "move"
)

// PatchOp represents a single patch operation.
type PatchOp struct {
	Op       OpType            `json:"op"`
	ID       string            `json:"id,omitempty"`        // target node ID (update, remove, move)
	ParentID string            `json:"parent_id,omitempty"` // for insert and move
	AfterID  string            `json:"after_id,omitempty"`  // positional sibling for insert and move
	Props    map[string]any    `json:"props,omitempty"`     // for update and insert
	Scripts  map[string]string `json:"scripts,omitempty"`
	Computed map[string]string `json:"computed,omitempty"`
	NodeType NodeType          `json:"type,omitempty"` // for insert
	Node     *NodeSpec         `json:"node,omitempty"` // full node spec for insert (alternative to flat fields)
}

// NodeSpec is a JSON-friendly node specification for insert operations.
type NodeSpec struct {
	ID       string            `json:"id"`
	Type     NodeType          `json:"type"`
	Props    map[string]any    `json:"props,omitempty"`
	Scripts  map[string]string `json:"scripts,omitempty"`
	Computed map[string]string `json:"computed,omitempty"`
	Children []*NodeSpec       `json:"children,omitempty"`
}

// PatchError is returned when a patch operation fails.
type PatchError struct {
	OpIndex int    `json:"op_index"`
	Op      OpType `json:"op"`
	Message string `json:"message"`
}

func (e *PatchError) Error() string {
	return fmt.Sprintf("patch op[%d] %s: %s", e.OpIndex, e.Op, e.Message)
}

// Patch applies an ordered list of operations to the tree atomically.
// If any operation fails, all changes are rolled back.
func (t *Tree) Patch(ops []PatchOp) error {
	if len(ops) == 0 {
		return nil
	}

	// Take a snapshot of the tree for rollback.
	backup := t.snapshot()

	for i, op := range ops {
		var err error
		switch op.Op {
		case OpUpdate:
			err = t.applyUpdate(op)
		case OpInsert:
			err = t.applyInsert(op)
		case OpRemove:
			err = t.applyRemove(op)
		case OpMove:
			err = t.applyMove(op)
		default:
			err = fmt.Errorf("unknown op type %q", op.Op)
		}
		if err != nil {
			// Rollback.
			t.restore(backup)
			return &PatchError{OpIndex: i, Op: op.Op, Message: err.Error()}
		}
	}
	return nil
}

func (t *Tree) applyUpdate(op PatchOp) error {
	if op.ID == "" {
		return fmt.Errorf("update: node ID is required")
	}
	node := t.Find(op.ID)
	if node == nil {
		return fmt.Errorf("node %q not found", op.ID)
	}
	// Merge props (not replace).
	for k, v := range op.Props {
		node.Props[k] = v
	}
	// Merge scripts.
	for k, v := range op.Scripts {
		node.Scripts[k] = v
	}
	// Merge computed.
	for k, v := range op.Computed {
		node.Computed[k] = v
	}
	return nil
}

func (t *Tree) applyInsert(op PatchOp) error {
	if op.ParentID == "" {
		return fmt.Errorf("insert: parent_id is required")
	}

	var node *Node
	if op.Node != nil {
		// Build node from spec.
		var err error
		node, err = buildNodeFromSpec(op.Node)
		if err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	} else {
		// Build from flat fields.
		if op.ID == "" {
			return fmt.Errorf("insert: node ID is required")
		}
		if op.NodeType == "" {
			return fmt.Errorf("insert node %q: type is required", op.ID)
		}
		var err error
		node, err = NewNode(op.ID, op.NodeType)
		if err != nil {
			return err
		}
		for k, v := range op.Props {
			node.Props[k] = v
		}
		for k, v := range op.Scripts {
			node.Scripts[k] = v
		}
		for k, v := range op.Computed {
			node.Computed[k] = v
		}
	}

	return t.Insert(op.ParentID, node, op.AfterID)
}

func (t *Tree) applyRemove(op PatchOp) error {
	if op.ID == "" {
		return fmt.Errorf("remove: node ID is required")
	}
	_, err := t.Remove(op.ID)
	return err
}

func (t *Tree) applyMove(op PatchOp) error {
	if op.ID == "" {
		return fmt.Errorf("move: node ID is required")
	}
	if op.ParentID == "" {
		return fmt.Errorf("move node %q: parent_id is required", op.ID)
	}
	return t.Move(op.ID, op.ParentID, op.AfterID)
}

// snapshot creates a deep copy of the tree for rollback.
func (t *Tree) snapshot() *treeSnapshot {
	return &treeSnapshot{
		root: t.Root.deepCopy(),
	}
}

// restore replaces the tree state from a snapshot.
func (t *Tree) restore(snap *treeSnapshot) {
	t.Root = snap.root
	t.index = make(map[string]*Node)
	_ = t.rebuildIndex(t.Root)
}

type treeSnapshot struct {
	root *Node
}

// buildNodeFromSpec recursively builds a Node tree from a NodeSpec.
func buildNodeFromSpec(spec *NodeSpec) (*Node, error) {
	node, err := NewNode(spec.ID, spec.Type)
	if err != nil {
		return nil, err
	}
	for k, v := range spec.Props {
		node.Props[k] = v
	}
	for k, v := range spec.Scripts {
		node.Scripts[k] = v
	}
	for k, v := range spec.Computed {
		node.Computed[k] = v
	}
	for _, childSpec := range spec.Children {
		child, err := buildNodeFromSpec(childSpec)
		if err != nil {
			return nil, err
		}
		child.parent = node
		node.Children = append(node.Children, child)
	}
	return node, nil
}

// Replace removes all children of the target node and replaces them
// with the given new subtree spec. If targetID matches the root,
// the root's children are replaced.
func (t *Tree) Replace(targetID string, newChildren []*NodeSpec) error {
	target := t.Find(targetID)
	if target == nil {
		return fmt.Errorf("replace: node %q not found", targetID)
	}

	// Build new nodes first and check for ID collisions.
	var newNodes []*Node
	newIDs := make(map[string]bool)
	for _, spec := range newChildren {
		node, err := buildNodeFromSpec(spec)
		if err != nil {
			return fmt.Errorf("replace: %w", err)
		}
		if err := collectIDs(node, newIDs); err != nil {
			return fmt.Errorf("replace: %w", err)
		}
		newNodes = append(newNodes, node)
	}

	// Collect IDs that will be removed (the target's current children).
	removedIDs := make(map[string]bool)
	for _, child := range target.Children {
		collectAllIDs(child, removedIDs)
	}

	// Check for collisions with existing tree (excluding removed nodes and the target).
	for id := range newIDs {
		if id == targetID {
			return fmt.Errorf("replace: new subtree contains ID %q which conflicts with target", id)
		}
		if _, exists := t.index[id]; exists && !removedIDs[id] {
			return fmt.Errorf("replace: duplicate node ID %q", id)
		}
	}

	// Remove old children from index.
	for _, child := range target.Children {
		t.removeFromIndex(child)
	}

	// Set new children.
	target.Children = newNodes
	for _, child := range newNodes {
		child.parent = target
		t.addToIndex(child)
	}

	return nil
}

// ReplaceTree replaces the entire tree with a new root spec.
func (t *Tree) ReplaceTree(spec *NodeSpec) error {
	root, err := buildNodeFromSpec(spec)
	if err != nil {
		return fmt.Errorf("replace tree: %w", err)
	}
	// Check for duplicate IDs within the new tree.
	ids := make(map[string]bool)
	if err := collectIDs(root, ids); err != nil {
		return fmt.Errorf("replace tree: %w", err)
	}
	t.Root = root
	t.index = make(map[string]*Node)
	return t.rebuildIndex(t.Root)
}

func collectIDs(n *Node, ids map[string]bool) error {
	if ids[n.ID] {
		return fmt.Errorf("duplicate node ID %q in subtree", n.ID)
	}
	ids[n.ID] = true
	for _, child := range n.Children {
		if err := collectIDs(child, ids); err != nil {
			return err
		}
	}
	return nil
}

func collectAllIDs(n *Node, ids map[string]bool) {
	ids[n.ID] = true
	for _, child := range n.Children {
		collectAllIDs(child, ids)
	}
}

// ParsePatchOps parses a JSON array of patch operations.
func ParsePatchOps(data []byte) ([]PatchOp, error) {
	var ops []PatchOp
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, fmt.Errorf("parse patch ops: %w", err)
	}
	return ops, nil
}
