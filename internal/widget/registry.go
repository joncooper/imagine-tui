package widget

import (
	"fmt"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// Registry maps node types to widget factories.
type Registry struct {
	factories map[dom.NodeType]func() Widget
}

// NewRegistry creates an empty widget registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[dom.NodeType]func() Widget),
	}
}

// Register associates a node type with a factory function.
// If the type is already registered, the old factory is replaced.
func (r *Registry) Register(nodeType dom.NodeType, factory func() Widget) {
	r.factories[nodeType] = factory
}

// Create returns a new widget instance for the given node type.
// Returns an error if the type is not registered.
func (r *Registry) Create(nodeType dom.NodeType) (Widget, error) {
	factory, ok := r.factories[nodeType]
	if !ok {
		return nil, fmt.Errorf("no widget registered for node type %q", nodeType)
	}
	return factory(), nil
}

// Has returns true if a widget factory is registered for the given type.
func (r *Registry) Has(nodeType dom.NodeType) bool {
	_, ok := r.factories[nodeType]
	return ok
}
