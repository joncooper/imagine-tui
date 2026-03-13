// Package mcp implements the MCP server and tool handlers (patch, replace,
// await_event, snapshot, restore, query). Depends on dom/.
//
// Built on the official MCP Go SDK (github.com/modelcontextprotocol/go-sdk).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolHandler is the signature for a tool handler function.
type ToolHandler = func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Server wraps the MCP server with a DOM tree, event queue, and snapshot store.
type Server struct {
	mu       sync.RWMutex
	tree     *dom.Tree
	events   *dom.EventQueue
	snaps    *dom.SnapshotStore
	srv      *mcp.Server
	handlers map[string]ToolHandler // tool name -> handler, for direct invocation
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
		handlers: make(map[string]ToolHandler),
		shutdown: make(chan struct{}),
	}

	s.srv = mcp.NewServer(&mcp.Implementation{
		Name:    "imagine-tui",
		Version: "0.1.0",
	}, nil)

	s.registerTools()
	return s, nil
}

// MCPServer returns the underlying MCP server for transport binding.
func (s *Server) MCPServer() *mcp.Server {
	return s.srv
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

// RLock acquires a read lock on the server's state. Use this when reading
// the DOM tree from a goroutine that may run concurrently with MCP mutations.
func (s *Server) RLock() {
	s.mu.RLock()
}

// RUnlock releases the read lock.
func (s *Server) RUnlock() {
	s.mu.RUnlock()
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

// --- Typed input structs ---

type patchInput struct {
	Ops json.RawMessage `json:"ops"`
}

type replaceInput struct {
	TargetID string          `json:"target_id,omitempty"`
	Tree     json.RawMessage `json:"tree,omitempty"`
	Children json.RawMessage `json:"children,omitempty"`
}

type awaitEventInput struct {
	TimeoutMs  int      `json:"timeout_ms,omitempty"`
	Filter     []string `json:"filter,omitempty"`
	DebounceMs int      `json:"debounce_ms,omitempty"`
}

type nameInput struct {
	Name string `json:"name"`
}

type queryInput struct {
	IDs []string `json:"ids"`
}

// CallTool invokes a tool handler directly by name. Useful for integration
// testing without going through the full MCP transport.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal args: %w", err)
	}

	handler, ok := s.handlers[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %q", name)
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      name,
			Arguments: argsJSON,
		},
	}

	return handler(ctx, req)
}

// --- Tool registration ---

func (s *Server) registerTools() {
	s.addTool(patchTool(), s.handlePatch)
	s.addTool(replaceTool(), s.handleReplace)
	s.addTool(awaitEventTool(), s.handleAwaitEvent)
	s.addTool(snapshotTool(), s.handleSnapshot)
	s.addTool(restoreTool(), s.handleRestore)
	s.addTool(queryTool(), s.handleQuery)
}

func (s *Server) addTool(tool *mcp.Tool, handler ToolHandler) {
	s.srv.AddTool(tool, handler)
	s.handlers[tool.Name] = handler
}

func patchTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "patch",
		Description: "Apply an ordered list of atomic operations to the TUI DOM tree",
		InputSchema: inputSchema(
			prop("ops", "array", "Ordered list of patch operations (update, insert, remove, move)"),
			"ops",
		),
	}
}

func replaceTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "replace",
		Description: "Replace an entire subtree or the whole tree",
		InputSchema: inputSchema(
			mergeProps(
				prop("target_id", "string", "ID of the node whose children will be replaced. If omitted, replaces the entire tree."),
				prop("tree", "object", "Full tree spec for whole-tree replacement (when target_id is omitted)"),
				prop("children", "array", "Array of node specs to replace the target's children"),
			),
		),
	}
}

func awaitEventTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "await_event",
		Description: "Long-poll: block until a Claude-routed event fires, then return it with context",
		InputSchema: inputSchema(
			mergeProps(
				prop("timeout_ms", "number", "Return timeout:true after this many milliseconds if no event fires"),
				prop("filter", "array", "Array of node IDs to listen to. Events from other nodes are held."),
				prop("debounce_ms", "number", "Coalesce rapid events within this window (ms)"),
			),
		),
	}
}

func snapshotTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "snapshot",
		Description: "Save the current DOM state under a name",
		InputSchema: inputSchema(
			prop("name", "string", "Name for this snapshot"),
			"name",
		),
	}
}

func restoreTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "restore",
		Description: "Restore the DOM to a previously saved snapshot",
		InputSchema: inputSchema(
			prop("name", "string", "Name of the snapshot to restore"),
			"name",
		),
	}
}

func queryTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "query",
		Description: "Read back current state of specific nodes",
		InputSchema: inputSchema(
			prop("ids", "array", "Array of node IDs to query"),
			"ids",
		),
	}
}

// --- Schema helpers ---

func prop(name, typ, desc string) map[string]any {
	return map[string]any{
		name: map[string]any{
			"type":        typ,
			"description": desc,
		},
	}
}

func mergeProps(props ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for _, p := range props {
		for k, v := range p {
			merged[k] = v
		}
	}
	return merged
}

func inputSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// --- Tool handlers ---

func (s *Server) handlePatch(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input patchInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if len(input.Ops) == 0 {
		return errResult("missing required parameter: ops"), nil
	}

	ops, err := dom.ParsePatchOps(input.Ops)
	if err != nil {
		return errResult(fmt.Sprintf("invalid ops: %v", err)), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.tree.Patch(ops); err != nil {
		return errResult(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true})
}

func (s *Server) handleReplace(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input replaceInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if input.TargetID == "" {
		// Whole-tree replacement.
		if len(input.Tree) == 0 {
			return errResult("replace requires either target_id or tree"), nil
		}
		var spec dom.NodeSpec
		if err := json.Unmarshal(input.Tree, &spec); err != nil {
			return errResult(fmt.Sprintf("invalid tree spec: %v", err)), nil
		}
		if err := s.tree.ReplaceTree(&spec); err != nil {
			return errResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"ok": true})
	}

	// Subtree replacement.
	if len(input.Children) == 0 {
		return errResult("replace with target_id requires children"), nil
	}
	var specs []*dom.NodeSpec
	if err := json.Unmarshal(input.Children, &specs); err != nil {
		return errResult(fmt.Sprintf("invalid children: %v", err)), nil
	}
	if err := s.tree.Replace(input.TargetID, specs); err != nil {
		return errResult(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true})
}

func (s *Server) handleAwaitEvent(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input awaitEventInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}

	// Build dequeue context with timeout.
	deqCtx := ctx
	if input.TimeoutMs > 0 {
		var cancel context.CancelFunc
		deqCtx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	opts := &dom.DequeueOpts{
		Filter:     input.Filter,
		DebounceMs: input.DebounceMs,
	}

	evt, err := s.events.Dequeue(deqCtx, opts)
	if err != nil {
		// Check if this was a timeout.
		if deqCtx.Err() != nil && input.TimeoutMs > 0 {
			return jsonResult(map[string]any{"timeout": true})
		}
		return errResult(fmt.Sprintf("await_event: %v", err)), nil
	}

	// Enrich with DOM summary if not already present.
	if evt.DOMSummary == "" {
		s.mu.Lock()
		evt.DOMSummary = s.tree.Summary()
		s.mu.Unlock()
	}

	return jsonResult(evt)
}

func (s *Server) handleSnapshot(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input nameInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if input.Name == "" {
		return errResult("missing required parameter: name"), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.snaps.Snapshot(input.Name, s.tree); err != nil {
		return errResult(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true, "name": input.Name})
}

func (s *Server) handleRestore(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input nameInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if input.Name == "" {
		return errResult("missing required parameter: name"), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.snaps.Restore(input.Name, s.tree); err != nil {
		return errResult(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true, "restored": input.Name})
}

func (s *Server) handleQuery(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input queryInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if len(input.IDs) == 0 {
		return errResult("missing required parameter: ids"), nil
	}

	s.mu.Lock()
	results, errs := s.tree.Query(input.IDs)
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

	return jsonResult(resp)
}

// --- Helpers ---

func unmarshalArgs(req *mcp.CallToolRequest, v any) error {
	if req.Params.Arguments == nil {
		return nil
	}
	return json.Unmarshal(req.Params.Arguments, v)
}

func errResult(msg string) *mcp.CallToolResult {
	r := &mcp.CallToolResult{}
	r.SetError(fmt.Errorf("%s", msg))
	return r
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errResult(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}
