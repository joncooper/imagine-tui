// Package mcp implements the MCP server and tool handlers (patch, replace,
// await_event, snapshot, restore, query). Depends on dom/.
//
// Built on the MCP Go SDK (github.com/mark3labs/mcp-go).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with a DOM tree, event queue, and snapshot store.
type Server struct {
	mu       sync.Mutex
	tree     *dom.Tree
	events   *dom.EventQueue
	snaps    *dom.SnapshotStore
	mcp      *server.MCPServer
	shutdown chan struct{}
}

// NewServer creates a new MCP server with all tool declarations registered.
// It initializes a minimal DOM tree with a single root container node.
func NewServer() (*Server, error) {
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		return nil, fmt.Errorf("create root node: %w", err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		return nil, fmt.Errorf("create tree: %w", err)
	}

	s := &Server{
		tree:     tree,
		events:   dom.NewEventQueue(),
		snaps:    dom.NewSnapshotStore(),
		shutdown: make(chan struct{}),
	}

	s.mcp = server.NewMCPServer(
		"imagine-tui",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	s.registerTools()
	return s, nil
}

// MCPServer returns the underlying MCP server for transport binding.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcp
}

// Tree returns the current DOM tree. Callers must not mutate without holding the lock.
func (s *Server) Tree() *dom.Tree {
	return s.tree
}

// Events returns the event queue.
func (s *Server) Events() *dom.EventQueue {
	return s.events
}

// Snapshots returns the snapshot store.
func (s *Server) Snapshots() *dom.SnapshotStore {
	return s.snaps
}

// Shutdown signals the server to stop. Closes the event queue.
func (s *Server) Shutdown() {
	select {
	case <-s.shutdown:
		// Already shut down.
	default:
		close(s.shutdown)
		s.events.Close()
	}
}

// IsShutdown returns true if the server has been shut down.
func (s *Server) IsShutdown() bool {
	select {
	case <-s.shutdown:
		return true
	default:
		return false
	}
}

func (s *Server) registerTools() {
	s.mcp.AddTool(patchTool(), s.handlePatch)
	s.mcp.AddTool(replaceTool(), s.handleReplace)
	s.mcp.AddTool(awaitEventTool(), s.handleAwaitEvent)
	s.mcp.AddTool(snapshotTool(), s.handleSnapshot)
	s.mcp.AddTool(restoreTool(), s.handleRestore)
	s.mcp.AddTool(queryTool(), s.handleQuery)
}

// --- Tool definitions ---

func patchTool() mcpsdk.Tool {
	return mcpsdk.NewTool("patch",
		mcpsdk.WithDescription("Apply an ordered list of atomic operations to the TUI DOM tree"),
		mcpsdk.WithArray("ops",
			mcpsdk.Required(),
			mcpsdk.Description("Ordered list of patch operations (update, insert, remove, move)"),
		),
	)
}

func replaceTool() mcpsdk.Tool {
	return mcpsdk.NewTool("replace",
		mcpsdk.WithDescription("Replace an entire subtree or the whole tree"),
		mcpsdk.WithString("target_id",
			mcpsdk.Description("ID of the node whose children will be replaced. If omitted, replaces the entire tree."),
		),
		mcpsdk.WithObject("tree",
			mcpsdk.Description("Full tree spec for whole-tree replacement (when target_id is omitted)"),
		),
		mcpsdk.WithArray("children",
			mcpsdk.Description("Array of node specs to replace the target's children"),
		),
	)
}

func awaitEventTool() mcpsdk.Tool {
	return mcpsdk.NewTool("await_event",
		mcpsdk.WithDescription("Long-poll: block until a Claude-routed event fires, then return it with context"),
		mcpsdk.WithNumber("timeout_ms",
			mcpsdk.Description("Return timeout:true after this many milliseconds if no event fires"),
		),
		mcpsdk.WithArray("filter",
			mcpsdk.Description("Array of node IDs to listen to. Events from other nodes are held."),
		),
		mcpsdk.WithNumber("debounce_ms",
			mcpsdk.Description("Coalesce rapid events within this window (ms)"),
		),
	)
}

func snapshotTool() mcpsdk.Tool {
	return mcpsdk.NewTool("snapshot",
		mcpsdk.WithDescription("Save the current DOM state under a name"),
		mcpsdk.WithString("name",
			mcpsdk.Required(),
			mcpsdk.Description("Name for this snapshot"),
		),
	)
}

func restoreTool() mcpsdk.Tool {
	return mcpsdk.NewTool("restore",
		mcpsdk.WithDescription("Restore the DOM to a previously saved snapshot"),
		mcpsdk.WithString("name",
			mcpsdk.Required(),
			mcpsdk.Description("Name of the snapshot to restore"),
		),
	)
}

func queryTool() mcpsdk.Tool {
	return mcpsdk.NewTool("query",
		mcpsdk.WithDescription("Read back current state of specific nodes"),
		mcpsdk.WithArray("ids",
			mcpsdk.Required(),
			mcpsdk.Description("Array of node IDs to query"),
		),
	)
}

// --- Tool handlers ---

func (s *Server) handlePatch(_ context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.IsShutdown() {
		return mcpsdk.NewToolResultError("server is shutting down"), nil
	}

	args := request.GetArguments()
	opsRaw, ok := args["ops"]
	if !ok {
		return mcpsdk.NewToolResultError("missing required parameter: ops"), nil
	}

	// Marshal back to JSON so we can use ParsePatchOps.
	opsJSON, err := json.Marshal(opsRaw)
	if err != nil {
		return mcpsdk.NewToolResultError(fmt.Sprintf("invalid ops: %v", err)), nil
	}

	ops, err := dom.ParsePatchOps(opsJSON)
	if err != nil {
		return mcpsdk.NewToolResultError(fmt.Sprintf("invalid ops: %v", err)), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.tree.Patch(ops); err != nil {
		return mcpsdk.NewToolResultError(err.Error()), nil
	}

	return resultJSON(map[string]any{"ok": true})
}

func (s *Server) handleReplace(_ context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.IsShutdown() {
		return mcpsdk.NewToolResultError("server is shutting down"), nil
	}

	args := request.GetArguments()
	targetID, _ := args["target_id"].(string)

	s.mu.Lock()
	defer s.mu.Unlock()

	if targetID == "" {
		// Whole-tree replacement.
		treeRaw, ok := args["tree"]
		if !ok {
			return mcpsdk.NewToolResultError("replace requires either target_id or tree"), nil
		}
		treeJSON, err := json.Marshal(treeRaw)
		if err != nil {
			return mcpsdk.NewToolResultError(fmt.Sprintf("invalid tree spec: %v", err)), nil
		}
		var spec dom.NodeSpec
		if err := json.Unmarshal(treeJSON, &spec); err != nil {
			return mcpsdk.NewToolResultError(fmt.Sprintf("invalid tree spec: %v", err)), nil
		}
		if err := s.tree.ReplaceTree(&spec); err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		return resultJSON(map[string]any{"ok": true})
	}

	// Subtree replacement.
	childrenRaw, ok := args["children"]
	if !ok {
		return mcpsdk.NewToolResultError("replace with target_id requires children"), nil
	}
	childrenJSON, err := json.Marshal(childrenRaw)
	if err != nil {
		return mcpsdk.NewToolResultError(fmt.Sprintf("invalid children: %v", err)), nil
	}
	var specs []*dom.NodeSpec
	if err := json.Unmarshal(childrenJSON, &specs); err != nil {
		return mcpsdk.NewToolResultError(fmt.Sprintf("invalid children: %v", err)), nil
	}
	if err := s.tree.Replace(targetID, specs); err != nil {
		return mcpsdk.NewToolResultError(err.Error()), nil
	}

	return resultJSON(map[string]any{"ok": true})
}

func (s *Server) handleAwaitEvent(ctx context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.IsShutdown() {
		return mcpsdk.NewToolResultError("server is shutting down"), nil
	}

	args := request.GetArguments()

	// Parse timeout.
	timeoutMs := 0
	if v, ok := args["timeout_ms"]; ok {
		if f, ok := v.(float64); ok {
			timeoutMs = int(f)
		}
	}

	// Parse filter.
	var filter []string
	if v, ok := args["filter"]; ok {
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if str, ok := item.(string); ok {
					filter = append(filter, str)
				}
			}
		}
	}

	// Parse debounce.
	debounceMs := 0
	if v, ok := args["debounce_ms"]; ok {
		if f, ok := v.(float64); ok {
			debounceMs = int(f)
		}
	}

	// Build dequeue context with timeout.
	deqCtx := ctx
	if timeoutMs > 0 {
		var cancel context.CancelFunc
		deqCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}

	opts := &dom.DequeueOpts{
		Filter:     filter,
		DebounceMs: debounceMs,
	}

	evt, err := s.events.Dequeue(deqCtx, opts)
	if err != nil {
		// Check if this was a timeout.
		if deqCtx.Err() != nil && timeoutMs > 0 {
			return resultJSON(map[string]any{"timeout": true})
		}
		return mcpsdk.NewToolResultError(fmt.Sprintf("await_event: %v", err)), nil
	}

	// Enrich with DOM summary if not already present.
	if evt.DOMSummary == "" {
		s.mu.Lock()
		evt.DOMSummary = s.tree.Summary()
		s.mu.Unlock()
	}

	return resultJSON(evt)
}

func (s *Server) handleSnapshot(_ context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.IsShutdown() {
		return mcpsdk.NewToolResultError("server is shutting down"), nil
	}

	name, _ := request.GetArguments()["name"].(string)
	if name == "" {
		return mcpsdk.NewToolResultError("missing required parameter: name"), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.snaps.Snapshot(name, s.tree); err != nil {
		return mcpsdk.NewToolResultError(err.Error()), nil
	}

	return resultJSON(map[string]any{"ok": true, "name": name})
}

func (s *Server) handleRestore(_ context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.IsShutdown() {
		return mcpsdk.NewToolResultError("server is shutting down"), nil
	}

	name, _ := request.GetArguments()["name"].(string)
	if name == "" {
		return mcpsdk.NewToolResultError("missing required parameter: name"), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.snaps.Restore(name, s.tree); err != nil {
		return mcpsdk.NewToolResultError(err.Error()), nil
	}

	return resultJSON(map[string]any{"ok": true, "restored": name})
}

func (s *Server) handleQuery(_ context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.IsShutdown() {
		return mcpsdk.NewToolResultError("server is shutting down"), nil
	}

	args := request.GetArguments()
	idsRaw, ok := args["ids"]
	if !ok {
		return mcpsdk.NewToolResultError("missing required parameter: ids"), nil
	}

	var ids []string
	if arr, ok := idsRaw.([]any); ok {
		for _, item := range arr {
			if str, ok := item.(string); ok {
				ids = append(ids, str)
			}
		}
	}

	s.mu.Lock()
	results, errs := s.tree.Query(ids)
	s.mu.Unlock()

	resp := map[string]any{
		"results": results,
	}
	if len(errs) > 0 {
		errStrs := make([]string, len(errs))
		for i, e := range errs {
			errStrs[i] = e.Error()
		}
		resp["errors"] = errStrs
	}

	return resultJSON(resp)
}

// --- Helpers ---

func resultJSON(v any) (*mcpsdk.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcpsdk.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcpsdk.NewToolResultText(string(data)), nil
}
