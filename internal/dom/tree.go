package dom

import (
	"fmt"
	"strings"
)

// Tree is the TUI DOM tree. It maintains a root node and an index
// for O(1) lookups by ID.
type Tree struct {
	Root  *Node
	index map[string]*Node
}

// NewTree creates a new tree with the given root node.
func NewTree(root *Node) (*Tree, error) {
	if root == nil {
		return nil, fmt.Errorf("tree root must not be nil")
	}
	t := &Tree{
		Root:  root,
		index: make(map[string]*Node),
	}
	// Index the root and all descendants.
	if err := t.rebuildIndex(root); err != nil {
		return nil, err
	}
	return t, nil
}

// Find returns the node with the given ID, or nil if not found.
func (t *Tree) Find(id string) *Node {
	return t.index[id]
}

// Insert adds a new node as a child of parentID.
// If afterID is non-empty, the node is placed after the sibling with that ID.
// If afterID is empty, the node is appended as the last child.
func (t *Tree) Insert(parentID string, node *Node, afterID string) error {
	if node == nil {
		return fmt.Errorf("insert: node must not be nil")
	}
	parent := t.Find(parentID)
	if parent == nil {
		return fmt.Errorf("insert node %q: parent %q not found", node.ID, parentID)
	}

	// Check for duplicate IDs in the new subtree against existing tree.
	if err := t.checkDuplicateIDs(node); err != nil {
		return fmt.Errorf("insert node %q: %w", node.ID, err)
	}

	if afterID != "" {
		idx := childIndex(parent, afterID)
		if idx < 0 {
			return fmt.Errorf("insert node %q: sibling %q not found in parent %q", node.ID, afterID, parentID)
		}
		// Insert after the sibling.
		pos := idx + 1
		parent.Children = append(parent.Children, nil)
		copy(parent.Children[pos+1:], parent.Children[pos:])
		parent.Children[pos] = node
	} else {
		parent.Children = append(parent.Children, node)
	}

	node.parent = parent
	t.addToIndex(node)
	return nil
}

// Remove removes the node with the given ID and all its descendants.
// Returns the removed subtree. The root node cannot be removed.
func (t *Tree) Remove(id string) (*Node, error) {
	node := t.Find(id)
	if node == nil {
		return nil, fmt.Errorf("remove: node %q not found", id)
	}
	if node == t.Root {
		return nil, fmt.Errorf("remove: cannot remove root node %q", id)
	}
	parent := node.parent
	if parent == nil {
		return nil, fmt.Errorf("remove: node %q has no parent", id)
	}

	// Remove from parent's children.
	idx := childIndex(parent, id)
	if idx < 0 {
		return nil, fmt.Errorf("remove: node %q not found in parent's children", id)
	}
	parent.Children = append(parent.Children[:idx], parent.Children[idx+1:]...)

	// Remove from index.
	t.removeFromIndex(node)
	node.parent = nil
	return node, nil
}

// Move reparents a node under a new parent, optionally after a sibling.
func (t *Tree) Move(id string, newParentID string, afterID string) error {
	node := t.Find(id)
	if node == nil {
		return fmt.Errorf("move: node %q not found", id)
	}
	if node == t.Root {
		return fmt.Errorf("move: cannot move root node %q", id)
	}
	newParent := t.Find(newParentID)
	if newParent == nil {
		return fmt.Errorf("move node %q: new parent %q not found", id, newParentID)
	}

	// Check for cycle: newParent must not be a descendant of node.
	if t.isDescendant(newParent, node) {
		return fmt.Errorf("move node %q: new parent %q is a descendant (would create cycle)", id, newParentID)
	}

	// Remove from old parent.
	oldParent := node.parent
	if oldParent != nil {
		idx := childIndex(oldParent, id)
		if idx >= 0 {
			oldParent.Children = append(oldParent.Children[:idx], oldParent.Children[idx+1:]...)
		}
	}

	// Insert into new parent.
	if afterID != "" {
		idx := childIndex(newParent, afterID)
		if idx < 0 {
			// Undo: re-attach to old parent at end.
			if oldParent != nil {
				oldParent.Children = append(oldParent.Children, node)
			}
			return fmt.Errorf("move node %q: sibling %q not found in new parent %q", id, afterID, newParentID)
		}
		pos := idx + 1
		newParent.Children = append(newParent.Children, nil)
		copy(newParent.Children[pos+1:], newParent.Children[pos:])
		newParent.Children[pos] = node
	} else {
		newParent.Children = append(newParent.Children, node)
	}

	node.parent = newParent
	return nil
}

// Walk performs a depth-first traversal of the tree, calling fn for each node.
// If fn returns false, the walk stops.
func (t *Tree) Walk(fn func(n *Node) bool) {
	t.walkNode(t.Root, fn)
}

func (t *Tree) walkNode(n *Node, fn func(n *Node) bool) bool {
	if !fn(n) {
		return false
	}
	for _, child := range n.Children {
		if !t.walkNode(child, fn) {
			return false
		}
	}
	return true
}

// Summary returns a compact string representation of the tree structure.
// Format: "root > child1 + child2(grandchild1, grandchild2) + child3"
func (t *Tree) Summary() string {
	if t.Root == nil {
		return ""
	}
	return nodeSummary(t.Root)
}

func nodeSummary(n *Node) string {
	if len(n.Children) == 0 {
		return n.ID
	}
	parts := make([]string, len(n.Children))
	for i, child := range n.Children {
		parts[i] = nodeSummary(child)
	}
	childStr := strings.Join(parts, ", ")
	return fmt.Sprintf("%s(%s)", n.ID, childStr)
}

// isDescendant returns true if candidate is a descendant of ancestor.
func (t *Tree) isDescendant(candidate, ancestor *Node) bool {
	for cur := candidate; cur != nil; cur = cur.parent {
		if cur == ancestor {
			return true
		}
	}
	return false
}

// rebuildIndex adds a node and all its descendants to the index.
func (t *Tree) rebuildIndex(n *Node) error {
	if _, exists := t.index[n.ID]; exists {
		return fmt.Errorf("duplicate node ID %q", n.ID)
	}
	t.index[n.ID] = n
	for _, child := range n.Children {
		if err := t.rebuildIndex(child); err != nil {
			return err
		}
	}
	return nil
}

// addToIndex adds a node and all its descendants to the index.
func (t *Tree) addToIndex(n *Node) {
	t.index[n.ID] = n
	for _, child := range n.Children {
		t.addToIndex(child)
	}
}

// removeFromIndex removes a node and all its descendants from the index.
func (t *Tree) removeFromIndex(n *Node) {
	delete(t.index, n.ID)
	for _, child := range n.Children {
		t.removeFromIndex(child)
	}
}

// checkDuplicateIDs checks that no node ID in the subtree rooted at n
// already exists in the tree index.
func (t *Tree) checkDuplicateIDs(n *Node) error {
	if _, exists := t.index[n.ID]; exists {
		return fmt.Errorf("duplicate node ID %q", n.ID)
	}
	for _, child := range n.Children {
		if err := t.checkDuplicateIDs(child); err != nil {
			return err
		}
	}
	return nil
}

// RebuildFullIndex rebuilds the entire ID index from scratch by walking the tree.
func (t *Tree) RebuildFullIndex() error {
	t.index = make(map[string]*Node)
	return t.rebuildIndex(t.Root)
}

// childIndex returns the index of the child with the given ID in the parent's
// children slice, or -1 if not found.
func childIndex(parent *Node, childID string) int {
	for i, c := range parent.Children {
		if c.ID == childID {
			return i
		}
	}
	return -1
}
