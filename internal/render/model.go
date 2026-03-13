package render

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/widget"
)

// Model is the top-level BubbleTea model that wires together the DOM tree,
// widget renderers, event queue, and MCP server.
type Model struct {
	server  *imcp.Server
	widgets *widget.Tree
	focus   *FocusRing

	focusedID    string
	width        int
	height       int
	disconnected bool
}

// NewModel creates a new render Model backed by the given MCP server and widget registry.
func NewModel(srv *imcp.Server, registry *widget.Registry) Model {
	return Model{
		server:  srv,
		widgets: widget.NewTree(registry),
		focus:   &FocusRing{index: make(map[string]int)},
	}
}

// Init implements tea.Model. No startup command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Routes messages to the appropriate handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncState()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case DOMChangedMsg:
		m.syncState()
		// If focused node was removed, adjust focus.
		if m.focusedID != "" && !m.focus.Contains(m.focusedID) {
			m.focusedID = m.focus.Next("")
		}
		return m, nil

	case MCPDisconnectedMsg:
		m.disconnected = true
		return m, nil

	case MCPConnectedMsg:
		m.disconnected = false
		m.syncState()
		return m, nil

	case ShutdownMsg:
		m.server.Shutdown()
		return m, tea.Quit
	}

	return m, nil
}

// View implements tea.Model. Renders the DOM tree via the widget tree.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if m.disconnected {
		return m.renderDisconnected()
	}

	m.server.RLock()
	defer m.server.RUnlock()
	return m.widgets.Render(m.server.Tree(), m.width, m.height, m.focusedID)
}

// syncState synchronizes the widget tree and focus ring with the current DOM.
func (m *Model) syncState() {
	m.server.RLock()
	defer m.server.RUnlock()
	_ = m.widgets.Sync(m.server.Tree())
	m.focus = BuildFocusRing(m.server.Tree())
}

// handleKey processes keyboard input. Acquires a read lock on the server to
// protect against concurrent MCP mutations while reading the DOM tree.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.server.Shutdown()
		return m, tea.Quit

	default:
		m.server.RLock()
		defer m.server.RUnlock()

		switch msg.Type {
		case tea.KeyTab:
			m.focusedID = m.focusTab(true)
			return m, nil

		case tea.KeyShiftTab:
			m.focusedID = m.focusTab(false)
			return m, nil

		default:
			// Route to focused widget.
			if m.focusedID != "" {
				return m.routeKeyToWidget(msg)
			}
			return m, nil
		}
	}
}

// focusTab advances focus forward (Tab) or backward (Shift+Tab).
func (m *Model) focusTab(forward bool) string {
	tree := m.server.Tree()
	if forward {
		return m.focus.NextInTrap(m.focusedID, tree)
	}
	return m.focus.PrevInTrap(m.focusedID, tree)
}

// routeKeyToWidget sends a key message to the currently focused widget and
// processes any events it produces.
func (m Model) routeKeyToWidget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := m.widgets.Get(m.focusedID)
	if w == nil {
		return m, nil
	}

	node := m.server.Tree().Find(m.focusedID)
	if node == nil {
		return m, nil
	}

	result := w.Update(msg, node)

	// Process widget events.
	for _, evt := range result.Events {
		m.routeWidgetEvent(evt)
	}

	return m, nil
}

// routeWidgetEvent routes a widget event to the appropriate destination:
// scripts first, then if the event should be Claude-routed, enqueue it.
func (m *Model) routeWidgetEvent(evt widget.Event) {
	// Enqueue as a Claude-routed event.
	domEvt := &dom.Event{
		Type:   evt.Type,
		Source: evt.NodeID,
		Data:   evt.Data,
	}

	// Auto-collect context from siblings.
	domEvt.Context = m.collectContext(evt.NodeID)
	domEvt.DOMSummary = m.server.Tree().Summary()

	m.server.Events().Enqueue(domEvt)
}

// collectContext gathers sibling and parent node state for event context.
func (m *Model) collectContext(sourceID string) map[string]map[string]any {
	node := m.server.Tree().Find(sourceID)
	if node == nil || node.Parent() == nil {
		return nil
	}

	ctx := make(map[string]map[string]any)
	parent := node.Parent()
	for _, sibling := range parent.Children {
		if sibling.ID == sourceID {
			continue
		}
		// Include value and key props from siblings.
		props := make(map[string]any)
		if v, ok := sibling.GetProp("value"); ok {
			props["value"] = v
		}
		if v, ok := sibling.GetProp("text"); ok {
			props["text"] = v
		}
		if len(props) > 0 {
			ctx[sibling.ID] = props
		}
	}

	if len(ctx) == 0 {
		return nil
	}
	return ctx
}

// renderDisconnected shows a disconnect message centered in the viewport.
func (m Model) renderDisconnected() string {
	msg := "MCP server disconnected"
	pad := (m.width - len(msg)) / 2
	if pad < 0 {
		pad = 0
	}
	line := strings.Repeat(" ", pad) + msg
	lines := make([]string, m.height)
	mid := m.height / 2
	for i := range lines {
		if i == mid {
			lines[i] = line
		} else {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
