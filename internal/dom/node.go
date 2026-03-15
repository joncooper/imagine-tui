package dom

import (
	"fmt"
	"strings"
)

// NodeType represents the type of a DOM node (widget type).
type NodeType string

// Core v1 widget types.
const (
	TypeContainer NodeType = "container"
	TypeText      NodeType = "text"
	TypeInput     NodeType = "input"
	TypeTextarea  NodeType = "textarea"
	TypeSelect    NodeType = "select"
	TypeButton    NodeType = "button"
	TypeTable     NodeType = "table"
	TypeList      NodeType = "list"
	TypeDiff      NodeType = "diff"
	TypeLog       NodeType = "log"
	TypeCode      NodeType = "code"
)

// v2 extended types.
const (
	TypeTabs      NodeType = "tabs"
	TypeProgress  NodeType = "progress"
	TypeSpinner   NodeType = "spinner"
	TypeMarkdown  NodeType = "markdown"
	TypeSparkline NodeType = "sparkline"
	TypeTree      NodeType = "tree"
	TypeModal     NodeType = "modal"
	TypeForm      NodeType = "form"
)

// validTypes is the registry of known node types.
var validTypes = map[NodeType]bool{
	TypeContainer: true,
	TypeText:      true,
	TypeInput:     true,
	TypeTextarea:  true,
	TypeSelect:    true,
	TypeButton:    true,
	TypeTable:     true,
	TypeList:      true,
	TypeDiff:      true,
	TypeLog:       true,
	TypeCode:      true,
	TypeTabs:      true,
	TypeProgress:  true,
	TypeSpinner:   true,
	TypeMarkdown:  true,
	TypeSparkline: true,
	TypeTree:      true,
	TypeModal:     true,
	TypeForm:      true,
}

// IsValidType returns true if the given node type is registered.
func IsValidType(t NodeType) bool {
	return validTypes[t]
}

// Node represents a single node in the TUI DOM tree.
type Node struct {
	ID       string
	Type     NodeType
	Props    map[string]any
	Children []*Node
	Scripts  map[string]string // hook name -> script body
	Computed map[string]string // prop name -> script expression
	parent   *Node
}

// Parent returns the node's parent, or nil if it is the root.
func (n *Node) Parent() *Node {
	return n.parent
}

// NewNode creates a new Node with the given ID and type after validation.
// Returns an error if the ID is empty, contains whitespace, or the type is unknown.
func NewNode(id string, nodeType NodeType) (*Node, error) {
	if err := ValidateID(id); err != nil {
		return nil, err
	}
	if !IsValidType(nodeType) {
		return nil, fmt.Errorf("node %q: unknown type %q", id, nodeType)
	}
	return &Node{
		ID:       id,
		Type:     nodeType,
		Props:    make(map[string]any),
		Scripts:  make(map[string]string),
		Computed: make(map[string]string),
	}, nil
}

// ValidateID checks that a node ID is non-empty and contains no whitespace.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("node ID must not be empty")
	}
	if strings.ContainsAny(id, " \t\n\r") {
		return fmt.Errorf("node %q: ID must not contain whitespace", id)
	}
	return nil
}

// SetProp sets a property on the node.
func (n *Node) SetProp(key string, value any) {
	n.Props[key] = value
}

// GetProp returns a property value and whether it exists.
func (n *Node) GetProp(key string) (any, bool) {
	v, ok := n.Props[key]
	return v, ok
}

// deepCopy returns a deep copy of the node and all its descendants.
// Parent pointers are set correctly within the copied subtree.
// The returned root's parent is nil.
func (n *Node) deepCopy() *Node {
	if n == nil {
		return nil
	}
	cp := &Node{
		ID:       n.ID,
		Type:     n.Type,
		Props:    copyMap(n.Props),
		Scripts:  copyStringMap(n.Scripts),
		Computed: copyStringMap(n.Computed),
	}
	for _, child := range n.Children {
		childCp := child.deepCopy()
		childCp.parent = cp
		cp.Children = append(cp.Children, childCp)
	}
	return cp
}

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = deepCopyValue(v)
	}
	return cp
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return copyMap(val)
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = deepCopyValue(item)
		}
		return cp
	default:
		return v // primitives are immutable
	}
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
