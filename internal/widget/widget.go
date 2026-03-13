// Package widget implements Lip Gloss renderers for each TUI widget type.
// Depends on dom/.
package widget

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// Widget is the contract every widget type implements.
// One instance is created per DOM node and persists for the node's lifetime.
type Widget interface {
	// Init is called once when the widget instance is created for a node.
	// The widget reads initial props and sets up internal state.
	Init(node *dom.Node)

	// Update handles a terminal input event (keypress, mouse).
	// It mutates widget-internal state and returns any DOM events produced.
	// The node is passed so the widget can write back state (e.g., input value).
	Update(msg tea.Msg, node *dom.Node) UpdateResult

	// View renders the widget to a string given current node props,
	// pre-rendered child views (empty for leaf widgets), and a context
	// with allocated dimensions and focus state.
	View(node *dom.Node, children []RenderedChild, ctx ViewContext) string

	// Layout computes the dimensions to allocate to each child node.
	// Only meaningful for container types; leaf widgets return nil.
	Layout(node *dom.Node, ctx ViewContext) []ChildConstraint
}

// UpdateResult is returned by Widget.Update.
type UpdateResult struct {
	Consumed bool    // true if the event was handled (stop bubbling)
	Events   []Event // events produced (routed by the runtime)
}

// Event is an event produced by a widget during Update.
type Event struct {
	Type   string         // "change", "submit", "click", "select", etc.
	NodeID string         // source node ID
	Data   map[string]any // event payload
}

// ViewContext provides rendering context to a widget's View method.
type ViewContext struct {
	Width   int    // allocated width in columns
	Height  int    // allocated height in rows (0 = unconstrained)
	Focused bool   // whether this node currently has focus
	Theme   *Theme // style token resolver
}

// ChildConstraint describes the dimensions allocated to a child by its parent.
type ChildConstraint struct {
	Width  int // allocated width for this child (0 = unconstrained)
	Height int // allocated height for this child (0 = unconstrained)
}

// RenderedChild holds a child's pre-rendered output for container composition.
type RenderedChild struct {
	NodeID string // which child this is
	View   string // pre-rendered output
}
