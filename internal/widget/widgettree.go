package widget

import (
	"io"
	"log/slog"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// Tree manages widget instances for a DOM tree.
// Each DOM node gets one widget instance, created on first sync and
// preserved across subsequent syncs as long as the node remains in the tree.
type Tree struct {
	registry  *Registry
	instances map[string]Widget // keyed by node ID
	logger    *slog.Logger
}

// NewTree creates a Tree backed by the given registry.
func NewTree(registry *Registry) *Tree {
	return &Tree{
		registry:  registry,
		instances: make(map[string]Widget),
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// SetLogger sets the structured logger for the widget tree.
func (wt *Tree) SetLogger(l *slog.Logger) {
	wt.logger = l
}

// Sync walks the DOM tree and ensures every node has a widget instance.
// New instances are created and Init'd; instances for removed nodes are deleted.
// Unknown widget types are skipped with a warning rather than aborting the walk.
func (wt *Tree) Sync(tree *dom.Tree) error {
	live := make(map[string]bool)
	created := 0
	skipped := 0

	tree.Walk(func(n *dom.Node) bool {
		live[n.ID] = true
		if _, exists := wt.instances[n.ID]; !exists {
			w, err := wt.registry.Create(n.Type)
			if err != nil {
				wt.logger.Warn("sync: skipping node with unknown type",
					"id", n.ID, "type", n.Type, "error", err)
				skipped++
				return true // continue walking — don't abort on unknown types
			}
			w.Init(n)
			wt.instances[n.ID] = w
			created++
		}
		return true
	})

	// Remove stale instances.
	removed := 0
	for id := range wt.instances {
		if !live[id] {
			delete(wt.instances, id)
			removed++
		}
	}

	wt.logger.Debug("sync complete",
		"created", created, "removed", removed, "skipped", skipped,
		"total_instances", len(wt.instances))

	return nil
}

// Get returns the widget instance for the given node ID, or nil if not found.
func (wt *Tree) Get(nodeID string) Widget {
	return wt.instances[nodeID]
}

// Render walks the DOM tree bottom-up, rendering each node via its widget.
// The two-pass approach: Layout (top-down) allocates child dimensions,
// then Render (bottom-up) composes the output.
func (wt *Tree) Render(tree *dom.Tree, width, height int, focusedID string) string {
	if width <= 0 {
		wt.logger.Warn("render: zero width", "width", width)
		return ""
	}
	theme := DefaultTheme()
	result := wt.renderNode(tree.Root, width, height, focusedID, theme)
	wt.logger.Debug("render complete", "output_len", len(result), "width", width, "height", height)
	return result
}

func (wt *Tree) renderNode(node *dom.Node, width, height int, focusedID string, theme *Theme) string {
	w := wt.instances[node.ID]
	if w == nil {
		wt.logger.Warn("render: no widget instance", "id", node.ID, "type", node.Type)
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
