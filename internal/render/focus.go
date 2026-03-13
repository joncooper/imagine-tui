package render

import "github.com/joncooper/imagine-tui/internal/dom"

// focusableTypes are node types that are focusable by default.
var focusableTypes = map[dom.NodeType]bool{
	dom.TypeInput:    true,
	dom.TypeTextarea: true,
	dom.TypeSelect:   true,
	dom.TypeButton:   true,
	dom.TypeTable:    true,
	dom.TypeList:     true,
	dom.TypeDiff:     true,
}

// FocusRing maintains an ordered list of focusable node IDs.
type FocusRing struct {
	IDs   []string
	index map[string]int // id → position in IDs
}

// BuildFocusRing walks the DOM tree depth-first and collects focusable node IDs.
// A node is focusable if its type is in focusableTypes, unless overridden by
// a "focusable" boolean prop on the node.
func BuildFocusRing(tree *dom.Tree) *FocusRing {
	ring := &FocusRing{index: make(map[string]int)}
	tree.Walk(func(n *dom.Node) bool {
		if isFocusable(n) {
			ring.index[n.ID] = len(ring.IDs)
			ring.IDs = append(ring.IDs, n.ID)
		}
		return true
	})
	return ring
}

// isFocusable returns whether a node should be in the focus ring.
func isFocusable(n *dom.Node) bool {
	// Explicit prop override takes precedence.
	if v, ok := n.GetProp("focusable"); ok {
		if b, isBool := v.(bool); isBool {
			return b
		}
	}
	return focusableTypes[n.Type]
}

// Contains returns true if the given ID is in the focus ring.
func (r *FocusRing) Contains(id string) bool {
	_, ok := r.index[id]
	return ok
}

// Next returns the next focusable ID after current. Wraps around.
// If current is not in the ring or empty, returns the first ID.
func (r *FocusRing) Next(current string) string {
	if len(r.IDs) == 0 {
		return ""
	}
	idx, ok := r.index[current]
	if !ok {
		return r.IDs[0]
	}
	return r.IDs[(idx+1)%len(r.IDs)]
}

// Prev returns the previous focusable ID before current. Wraps around.
// If current is not in the ring or empty, returns the last ID.
func (r *FocusRing) Prev(current string) string {
	if len(r.IDs) == 0 {
		return ""
	}
	idx, ok := r.index[current]
	if !ok {
		return r.IDs[len(r.IDs)-1]
	}
	return r.IDs[(idx-1+len(r.IDs))%len(r.IDs)]
}

// NextInTrap returns the next focusable ID, but only within the nearest
// focus-trapping ancestor container. If no trap is active, behaves like Next.
func (r *FocusRing) NextInTrap(current string, tree *dom.Tree) string {
	trapped := r.trappedIDs(current, tree)
	if trapped == nil {
		return r.Next(current)
	}
	return cycleNext(trapped, current)
}

// PrevInTrap returns the previous focusable ID within the nearest focus trap.
func (r *FocusRing) PrevInTrap(current string, tree *dom.Tree) string {
	trapped := r.trappedIDs(current, tree)
	if trapped == nil {
		return r.Prev(current)
	}
	return cyclePrev(trapped, current)
}

// trappedIDs returns the subset of ring IDs within the nearest focus-trapping
// ancestor of the current node. Returns nil if no trap is active.
func (r *FocusRing) trappedIDs(current string, tree *dom.Tree) []string {
	node := tree.Find(current)
	if node == nil {
		return nil
	}

	// Walk up to find nearest focus_trap ancestor.
	var trap *dom.Node
	for p := node.Parent(); p != nil; p = p.Parent() {
		if v, ok := p.GetProp("focus_trap"); ok {
			if b, isBool := v.(bool); isBool && b {
				trap = p
				break
			}
		}
	}
	if trap == nil {
		return nil
	}

	// Collect focusable IDs that are descendants of the trap.
	var trapped []string
	for _, id := range r.IDs {
		if isDescendantOf(tree, id, trap.ID) {
			trapped = append(trapped, id)
		}
	}
	if len(trapped) <= 1 {
		return nil // no point trapping with 0-1 nodes
	}
	return trapped
}

// isDescendantOf returns true if nodeID is a descendant of ancestorID.
func isDescendantOf(tree *dom.Tree, nodeID, ancestorID string) bool {
	node := tree.Find(nodeID)
	if node == nil {
		return false
	}
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.ID == ancestorID {
			return true
		}
	}
	return false
}

func cycleNext(ids []string, current string) string {
	for i, id := range ids {
		if id == current {
			return ids[(i+1)%len(ids)]
		}
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func cyclePrev(ids []string, current string) string {
	for i, id := range ids {
		if id == current {
			return ids[(i-1+len(ids))%len(ids)]
		}
	}
	if len(ids) > 0 {
		return ids[len(ids)-1]
	}
	return ""
}
