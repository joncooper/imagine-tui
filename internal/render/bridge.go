package render

import (
	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
)

// ProgramSender is the subset of tea.Program we need for sending messages.
// Using an interface allows testing without a real BubbleTea program.
type ProgramSender interface {
	Send(msg tea.Msg)
}

// Bridge connects the MCP server to the BubbleTea program.
// MCP tool handlers call bridge methods after mutations to notify the TUI.
type Bridge struct {
	srv     *imcp.Server
	program ProgramSender

	// OnMutation is an optional callback invoked before sending DOMChangedMsg.
	// Useful for syncing the widget tree or running scripts.
	OnMutation func()
}

// NewBridge creates a bridge between the MCP server and BubbleTea program.
func NewBridge(srv *imcp.Server, program ProgramSender) *Bridge {
	return &Bridge{
		srv:     srv,
		program: program,
	}
}

// Server returns the underlying MCP server.
func (b *Bridge) Server() *imcp.Server {
	return b.srv
}

// NotifyDOMChanged notifies the BubbleTea program that the DOM has changed.
// Called by MCP tool handlers after patch/replace/restore operations.
func (b *Bridge) NotifyDOMChanged() {
	if b.OnMutation != nil {
		b.OnMutation()
	}
	b.program.Send(DOMChangedMsg{})
}

// NotifyConnected notifies the BubbleTea program that a new MCP client connected.
func (b *Bridge) NotifyConnected(sessionID uint64) {
	b.program.Send(MCPConnectedMsg{SessionID: sessionID})
}

// NotifyDisconnected notifies the BubbleTea program that MCP has disconnected.
func (b *Bridge) NotifyDisconnected(sessionID uint64, err error) {
	b.program.Send(MCPDisconnectedMsg{SessionID: sessionID, Err: err})
}
