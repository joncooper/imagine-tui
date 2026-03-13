package widget

import (
	"fmt"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// WidgetTree manages widget instances for a DOM tree.
// Each DOM node gets one widget instance, created on first sync and
// preserved across subsequent syncs as long as the node remains in the tree.
type WidgetTree struct {
	registry  *Registry
	instances map[string]Widget // keyed by node ID
}

// NewWidgetTree creates a WidgetTree backed by the given registry.
func NewWidgetTree(registry *Registry) *WidgetTree {
	return &WidgetTree{
		registry:  registry,
		instances: make(map[string]Widget),
	}
}

// Sync walks the DOM tree and ensures every node has a widget instance.
// New instances are created and Init'd; instances for removed nodes are deleted.
func (wt *WidgetTree) Sync(tree *dom.Tree) error {
	live := make(map[string]bool)

	var syncErr error
	tree.Walk(func(n *dom.Node) bool {
		live[n.ID] = true
		if _, exists := wt.instances[n.ID]; !exists {
			w, err := wt.registry.Create(n.Type)
			if err != nil {
				syncErr = fmt.Errorf("sync node %q: %w", n.ID, err)
				return false
			}
			w.Init(n)
			wt.instances[n.ID] = w
		}
		return true
	})
	if syncErr != nil {
		return syncErr
	}

	// Remove stale instances.
	for id := range wt.instances {
		if !live[id] {
			delete(wt.instances, id)
		}
	}
	return nil
}

// Get returns the widget instance for the given node ID, or nil if not found.
func (wt *WidgetTree) Get(nodeID string) Widget {
	return wt.instances[nodeID]
}

// Render walks the DOM tree bottom-up, rendering each node via its widget.
// The two-pass approach: Layout (top-down) allocates child dimensions,
// then Render (bottom-up) composes the output.
func (wt *WidgetTree) Render(tree *dom.Tree, width, height int, focusedID string) string {
	if width <= 0 {
		return ""
	}
	theme := DefaultTheme()
	return wt.renderNode(tree.Root, width, height, focusedID, theme)
}

func (wt *WidgetTree) renderNode(node *dom.Node, width, height int, focusedID string, theme *Theme) string {
	w := wt.instances[node.ID]
	if w == nil {
		return ""
	}

	ctx := ViewContext{
		Width:   width,
		Height:  height,
		Focused: node.ID == focusedID,
		Theme:   theme,
	}

	// Layout pass: ask the widget for child constraints.
	constraints := w.Layout(node, ctx)

	// Render children.
	var children []RenderedChild
	for i, child := range node.Children {
		cw, ch := width, 0 // default: full parent width, unconstrained height
		if constraints != nil && i < len(constraints) {
			if constraints[i].Width > 0 {
				cw = constraints[i].Width
			}
			ch = constraints[i].Height
		}
		rendered := wt.renderNode(child, cw, ch, focusedID, theme)
		children = append(children, RenderedChild{
			NodeID: child.ID,
			View:   rendered,
		})
	}

	return w.View(node, children, ctx)
}
