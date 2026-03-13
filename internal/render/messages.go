package render

// DOMChangedMsg is sent to the BubbleTea program when the DOM tree has been
// mutated by an MCP tool call (patch, replace, restore). The Update loop
// re-syncs the widget tree and re-renders.
type DOMChangedMsg struct{}

// MCPDisconnectedMsg is sent when the MCP connection breaks (e.g., broken pipe).
type MCPDisconnectedMsg struct {
	Err error
}

// MCPConnectedMsg is sent when the MCP server is ready.
type MCPConnectedMsg struct{}

// ShutdownMsg requests a graceful shutdown.
type ShutdownMsg struct{}
