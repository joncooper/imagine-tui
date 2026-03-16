package render

import (
	"time"

	imcp "github.com/joncooper/imagine-tui/internal/mcp"
)

// DOMChangedMsg is sent to the BubbleTea program when the DOM tree has been
// mutated by an MCP tool call. The Update loop re-syncs the widget tree,
// re-renders, and may reinitialize focus for whole-tree mutations.
type DOMChangedMsg struct {
	MutationKind imcp.MutationKind
}

// MCPDisconnectedMsg is sent when the MCP connection breaks (e.g., broken pipe).
type MCPDisconnectedMsg struct {
	SessionID uint64
	Err       error
}

// MCPConnectedMsg is sent when the MCP server is ready.
type MCPConnectedMsg struct {
	SessionID uint64
}

// ShutdownMsg requests a graceful shutdown.
type ShutdownMsg struct{}

type scriptTimerMsg struct {
	Seq     uint64
	FiredAt time.Time
}
