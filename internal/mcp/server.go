// Package mcp implements the MCP server and tool handlers (patch, replace,
// await_event, snapshot, restore, query). Depends on dom/.
//
// Built on the official MCP Go SDK (github.com/modelcontextprotocol/go-sdk).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/joncooper/imagine-tui/internal/dom"
	"github.com/joncooper/imagine-tui/internal/widget"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolHandler is the signature for a tool handler function.
type ToolHandler = func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Server wraps the MCP server with a DOM tree, event queue, and snapshot store.
type Server struct {
	mu         sync.RWMutex
	tree       *dom.Tree
	events     *dom.EventQueue
	snaps      *dom.SnapshotStore
	srv        *mcp.Server
	handlers   map[string]ToolHandler // tool name -> handler, for direct invocation
	shutdown   chan struct{}
	onMutation func() // called after DOM-mutating operations (patch, replace, restore)
	logger     *slog.Logger
}

// SetLogger sets the structured logger for the server.
func (s *Server) SetLogger(l *slog.Logger) {
	s.logger = l
}

// SetOnMutation registers a callback that fires after successful DOM mutations.
// Used to notify BubbleTea of DOM changes so it can re-render.
func (s *Server) SetOnMutation(fn func()) {
	s.onMutation = fn
}

// notifyMutation calls the mutation callback if one is registered.
func (s *Server) notifyMutation() {
	if s.onMutation != nil {
		s.onMutation()
	}
}

// serverInstructions is sent to clients during MCP initialization.
// This is the primary way the LLM learns how to use imagine-tui — no CLAUDE.md required.
const serverInstructions = `imagine-tui is an MCP server that renders interactive terminal UIs.

## Getting started
1. Call describe_widgets to discover available widget types, their props, and events.
2. Call layout to define your UI structure as a tree of widgets.
3. Call set_items to populate list or table widgets with data.
4. Call await_event to wait for user interaction, then respond by updating the UI.

## Key tools
- describe_widgets: Discover widget types (call this first!)
- describe_scripting: Learn the reactive scripting system (hooks, $ API, computed props, emit, state, $.state, $('id').state)
- layout: Define UI structure (container, text, list, table, button, input, etc.)
- set_items / append_items / remove_items: Efficiently populate list, table, and log widgets with data
- patch: Incremental DOM updates (update props, insert/remove nodes)
- await_event: Long-poll for user events (click, select, submit, change)
- query: Read current node state
- snapshot / restore: Save and restore UI checkpoints

## Reactive scripting
If you add scripts or computed props, call describe_scripting before writing them.
It documents hook names, emit(), per-node state, and reactive state access via $.state and $('id').state.
Current state reactivity is top-level only: reads like state.count or $('store').state.count are tracked, but nested object mutation is not tracked yet.

## Widget overview
Widgets include: container (layout), text, list (navigable with up/down/enter),
table (sortable, expandable rows), button, input, textarea, select, code, log, diff.
Call describe_widgets for full details on any widget type.

## Data pattern
For data-heavy UIs, use layout + set_items instead of generating large JSON patches.
Define the structure once with layout, then send compact data arrays with set_items.
For log widgets, use append_items to add new lines without resending the entire array.
This is 10-20x fewer tokens than raw DOM manipulation.`

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
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	s.srv = mcp.NewServer(&mcp.Implementation{
		Name:    "imagine_tui",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})

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

// Lock acquires an exclusive write lock on the server's state. Use this when
// the BubbleTea goroutine needs to write to the DOM (e.g. widget SetProp) while
// MCP mutations may be running concurrently.
func (s *Server) Lock() {
	s.mu.Lock()
}

// Unlock releases the write lock.
func (s *Server) Unlock() {
	s.mu.Unlock()
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

type layoutInput struct {
	Tree json.RawMessage `json:"tree"`
}

type setItemsInput struct {
	Target string          `json:"target"`
	Items  json.RawMessage `json:"items"`
}

type removeItemsInput struct {
	Target string          `json:"target"`
	Keys   json.RawMessage `json:"keys"`
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
	s.addTool(layoutTool(), s.handleLayout)
	s.addTool(setItemsTool(), s.handleSetItems)
	s.addTool(appendItemsTool(), s.handleAppendItems)
	s.addTool(removeItemsTool(), s.handleRemoveItems)
	s.addTool(describeWidgetsTool(), s.handleDescribeWidgets)
	s.addTool(describeScriptingTool(), s.handleDescribeScripting)
}

func (s *Server) addTool(tool *mcp.Tool, handler ToolHandler) {
	s.srv.AddTool(tool, handler)
	s.handlers[tool.Name] = handler
}

func patchTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "patch",
		Description: "Apply an ordered list of atomic operations to the TUI DOM tree. Supported ops: update, insert, remove, move, append.",
		InputSchema: schema(map[string]JSONSchema{
			"ops": {Type: "array", Description: "Ordered list of patch operations (update, insert, remove, move, append). Append op: {op: \"append\", id: \"node-id\", prop: \"lines\", values: [...]}"},
		}, "ops"),
	}
}

func replaceTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "replace",
		Description: "Replace an entire subtree or the whole tree",
		InputSchema: schema(map[string]JSONSchema{
			"target_id": {Type: "string", Description: "ID of the node whose children will be replaced. If omitted, replaces the entire tree."},
			"tree":      {Type: "object", Description: "Full tree spec for whole-tree replacement (when target_id is omitted)"},
			"children":  {Type: "array", Description: "Array of node specs to replace the target's children"},
		}),
	}
}

func awaitEventTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "await_event",
		Description: "Long-poll: block until a Claude-routed event fires, then return it with context",
		InputSchema: schema(map[string]JSONSchema{
			"timeout_ms":  {Type: "number", Description: "Return timeout:true after this many milliseconds if no event fires"},
			"filter":      {Type: "array", Description: "Array of node IDs to listen to. Events from other nodes are held."},
			"debounce_ms": {Type: "number", Description: "Coalesce rapid events within this window (ms)"},
		}),
	}
}

func snapshotTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "snapshot",
		Description: "Save the current DOM state under a name",
		InputSchema: schema(map[string]JSONSchema{
			"name": {Type: "string", Description: "Name for this snapshot"},
		}, "name"),
	}
}

func restoreTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "restore",
		Description: "Restore the DOM to a previously saved snapshot",
		InputSchema: schema(map[string]JSONSchema{
			"name": {Type: "string", Description: "Name of the snapshot to restore"},
		}, "name"),
	}
}

func queryTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "query",
		Description: "Read back current state of specific nodes",
		InputSchema: schema(map[string]JSONSchema{
			"ids": {Type: "array", Description: "Array of node IDs to query"},
		}, "ids"),
	}
}

// --- Schema types ---

// JSONSchema is a minimal JSON Schema object for MCP tool input schemas.
type JSONSchema struct {
	Type        string                `json:"type"`
	Description string                `json:"description,omitempty"`
	Properties  map[string]JSONSchema `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
}

// schema builds a JSONSchema with the given properties and required fields.
// An empty properties map is always allocated so it serializes as {} not null.
func schema(props map[string]JSONSchema, required ...string) JSONSchema {
	if props == nil {
		props = map[string]JSONSchema{}
	}
	return JSONSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

// --- Tool response types ---

type okResult struct {
	OK bool `json:"ok"`
}

type okCountResult struct {
	OK        bool `json:"ok"`
	NodeCount int  `json:"node_count,omitempty"`
	Count     int  `json:"count,omitempty"`
	Removed   int  `json:"removed,omitempty"`
}

type okNameResult struct {
	OK       bool   `json:"ok"`
	Name     string `json:"name,omitempty"`
	Restored string `json:"restored,omitempty"`
}

type timeoutResult struct {
	Timeout bool `json:"timeout"`
}

type queryResult struct {
	Results map[string]*dom.QueryResult `json:"results"`
	Errors  []string                    `json:"errors,omitempty"`
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

	s.logger.Info("patch", "op_count", len(ops))

	s.mu.Lock()
	err = s.tree.Patch(ops)
	s.mu.Unlock()

	if err != nil {
		s.logger.Error("patch: failed", "error", err)
		return errResult(err.Error()), nil
	}

	s.logger.Info("patch: success")
	s.notifyMutation()
	return jsonResult(okResult{OK: true})
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

	if input.TargetID == "" {
		// Whole-tree replacement.
		if len(input.Tree) == 0 {
			s.mu.Unlock()
			return errResult("replace requires either target_id or tree"), nil
		}
		var spec dom.NodeSpec
		if err := json.Unmarshal(input.Tree, &spec); err != nil {
			s.mu.Unlock()
			s.logger.Error("replace: invalid tree spec", "error", err)
			return errResult(fmt.Sprintf("invalid tree spec: %v", err)), nil
		}
		childCount := len(spec.Children)
		s.logger.Info("replace: whole tree", "root_id", spec.ID, "root_type", spec.Type, "children", childCount)
		if err := s.tree.ReplaceTree(&spec); err != nil {
			s.mu.Unlock()
			s.logger.Error("replace: ReplaceTree failed", "error", err)
			return errResult(err.Error()), nil
		}
		nodeCount := 0
		s.tree.Walk(func(n *dom.Node) bool { nodeCount++; return true })
		summary := s.tree.Summary()
		s.mu.Unlock()
		s.logger.Info("replace: success", "node_count", nodeCount, "tree_summary", summary)
		s.notifyMutation()
		return jsonResult(okCountResult{OK: true, NodeCount: nodeCount})
	}

	// Subtree replacement.
	if len(input.Children) == 0 {
		s.mu.Unlock()
		return errResult("replace with target_id requires children"), nil
	}
	var specs []*dom.NodeSpec
	if err := json.Unmarshal(input.Children, &specs); err != nil {
		s.mu.Unlock()
		return errResult(fmt.Sprintf("invalid children: %v", err)), nil
	}
	if err := s.tree.Replace(input.TargetID, specs); err != nil {
		s.mu.Unlock()
		return errResult(err.Error()), nil
	}
	s.mu.Unlock()

	s.notifyMutation()
	return jsonResult(okResult{OK: true})
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
			return jsonResult(timeoutResult{Timeout: true})
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

	return jsonResult(okNameResult{OK: true, Name: input.Name})
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
	err := s.snaps.Restore(input.Name, s.tree)
	s.mu.Unlock()

	if err != nil {
		return errResult(err.Error()), nil
	}

	s.notifyMutation()
	return jsonResult(okNameResult{OK: true, Restored: input.Name})
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

	s.logger.Info("query", "ids", input.IDs, "tree_summary", s.tree.Summary())

	s.mu.Lock()
	results, errs := s.tree.Query(input.IDs)
	s.mu.Unlock()

	resp := queryResult{Results: results}
	if len(errs) > 0 {
		resp.Errors = make([]string, len(errs))
		for i, e := range errs {
			resp.Errors[i] = e.Error()
		}
		s.logger.Warn("query: some IDs not found", "errors", resp.Errors)
	}

	return jsonResult(resp)
}

// --- Template-driven data tools ---

func layoutTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "layout",
		Description: "Define or replace the UI structure. Container nodes may include an item_template prop for use with set_items/append_items. Idempotent.",
		InputSchema: schema(map[string]JSONSchema{
			"tree": {Type: "object", Description: "Full tree spec. Container nodes may include an item_template prop with {{key}} placeholders."},
		}, "tree"),
	}
}

func setItemsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "set_items",
		Description: "Populate a list, table, log, or templated container with data. For list nodes: items are {id, label, badge, style}. For table nodes: items are row objects. For log nodes: items are {text, level, timestamp}. For containers with item_template: items are expanded through the template. Replaces all existing items.",
		InputSchema: schema(map[string]JSONSchema{
			"target": {Type: "string", Description: "ID of the list node or container with item_template"},
			"items":  {Type: "array", Description: "Array of data objects. For lists: {id, label, badge, style}. For templates: keys map to {{key}} placeholders."},
		}, "target", "items"),
	}
}

func appendItemsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "append_items",
		Description: "Append data items to a list, table, log, or templated container without replacing existing items.",
		InputSchema: schema(map[string]JSONSchema{
			"target": {Type: "string", Description: "ID of the list node or container with item_template"},
			"items":  {Type: "array", Description: "Array of data objects to append"},
		}, "target", "items"),
	}
}

func removeItemsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "remove_items",
		Description: "Remove items from a list, table, log, or templated container. For list/table/log nodes: keys match item id fields. For containers: keys compute child IDs as {target}-{key}.",
		InputSchema: schema(map[string]JSONSchema{
			"target": {Type: "string", Description: "ID of the list node or container"},
			"keys":   {Type: "array", Description: "Array of item keys to remove"},
		}, "target", "keys"),
	}
}

func describeWidgetsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "describe_widgets",
		Description: "List available widget types with their props, events, and capabilities. Call this before building a UI to discover what widgets you can use. Optionally filter by type.",
		InputSchema: schema(map[string]JSONSchema{
			"type": {Type: "string", Description: "Optional: filter to a specific widget type (e.g. \"list\", \"table\")"},
		}),
	}
}

type describeWidgetsInput struct {
	Type string `json:"type"`
}

func (s *Server) handleDescribeWidgets(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input describeWidgetsInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}

	if input.Type != "" {
		catalog := widget.CatalogMap()
		info, ok := catalog[input.Type]
		if !ok {
			return errResult(fmt.Sprintf("unknown widget type %q", input.Type)), nil
		}
		return jsonResult(info)
	}

	return jsonResult(widget.Catalog())
}

func describeScriptingTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "describe_scripting",
		Description: "Describe the reactive scripting system: lifecycle hooks, the $ node API, computed props, emit, and state. Call this to learn how to add client-side logic to widgets.",
		InputSchema: schema(nil),
	}
}

// scriptingInfo is the static response for describe_scripting.
type scriptingInfo struct {
	Overview  string       `json:"overview"`
	Hooks     []hookInfo   `json:"hooks"`
	DollarAPI []apiEntry   `json:"dollar_api"`
	Globals   []apiEntry   `json:"globals"`
	Computed  computedInfo `json:"computed_props"`
	Sandbox   sandboxInfo  `json:"sandbox"`
	Examples  []example    `json:"examples"`
}

type hookInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type apiEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

type computedInfo struct {
	Description string `json:"description"`
	Declaration string `json:"declaration"`
}

type sandboxInfo struct {
	Description string   `json:"description"`
	Blocked     []string `json:"blocked"`
}

type example struct {
	Title string `json:"title"`
	Code  string `json:"code"`
}

func scriptingCatalog() *scriptingInfo {
	return &scriptingInfo{
		Overview: "Every DOM node can have JavaScript hooks and computed props. Scripts run in a sandboxed goja (ES5.1) VM. The only way to communicate outside the sandbox is via emit().",
		Hooks: []hookInfo{
			{Name: "on_mount", Description: "Runs once when the node enters the DOM"},
			{Name: "on_change", Description: "Runs when the node's value prop changes (inputs, selects)"},
			{Name: "on_submit", Description: "Runs when the user presses Enter on an input. Script runs first for instant feedback, then the event is still forwarded to the agent."},
			{Name: "on_event", Description: "Runs when a child node emits an event (bubbles up)"},
			{Name: "on_focus", Description: "Runs when the node receives focus"},
			{Name: "on_blur", Description: "Runs when the node loses focus"},
			{Name: "on_key", Description: "Runs on keypress when the node is focused"},
		},
		DollarAPI: []apiEntry{
			{Name: "$", Description: "Current node proxy. Read/write props: $.value, $.text, $.style, $.visible, $.rows, $.props.{key}"},
			{Name: "$.id", Description: "Node ID", ReadOnly: true},
			{Name: "$.type", Description: "Node type", ReadOnly: true},
			{Name: "$.state", Description: "Current node's persistent state object. Top-level reads in computed props are reactive; top-level writes mark the node dirty.", ReadOnly: true},
			{Name: "$.value", Description: "The node's value (for inputs, textareas, selects)"},
			{Name: "$.props", Description: "All props as an object — read or write individual keys"},
			{Name: "$.style", Description: "Style token string"},
			{Name: "$.text", Description: "Text content (alias for content prop)"},
			{Name: "$.visible", Description: "Show/hide the node (bool)"},
			{Name: "$.children", Description: "Child node proxies (read-only array)", ReadOnly: true},
			{Name: "$.rows", Description: "Table rows array"},
			{Name: "$('id')", Description: "Look up any node by ID and return a proxy with the same read/write API"},
			{Name: "$('id').state", Description: "Another node's persistent state object. Use this for shared local state across widgets; top-level reads and writes are reactive."},
		},
		Globals: []apiEntry{
			{Name: "emit('local', patchOps)", Description: "Apply a DOM patch synchronously from within the script (no MCP round-trip)"},
			{Name: "emit('agent', data)", Description: "Queue an event for the MCP client (delivered via await_event)"},
			{Name: "state", Description: "Per-node persistent JavaScript object — alias for $.state. Top-level reads in computed props are reactive; top-level writes dirty the node. nested object mutation is not tracked yet."},
			{Name: "event", Description: "The hook payload object (e.g., key info for on_key, value for on_change). Only defined during hook execution."},
			{Name: "debug(...args)", Description: "Log to the server's debug output (not visible in TUI)"},
			{Name: "setTimeout(fn, delayMs)", Description: "Schedule a one-shot callback owned by the current node. Delays above the runtime cap are rejected."},
			{Name: "setInterval(fn, delayMs)", Description: "Schedule a repeating callback owned by the current node. Use clearInterval(id) to stop it."},
			{Name: "clearTimeout(id)", Description: "Cancel a pending timeout by timer ID"},
			{Name: "clearInterval(id)", Description: "Cancel a pending interval by timer ID"},
		},
		Computed: computedInfo{
			Description: "Computed props are reactive expressions that auto-update when dependencies change. Declare them in the node's computed map. Dependencies are tracked automatically via $ access and top-level state reads such as state.count, $.state.count, and $('store').state.count. nested object mutation is not tracked yet.",
			Declaration: "In node spec: {\"computed\": {\"display_text\": \"return $.value.toUpperCase()\"}}",
		},
		Sandbox: sandboxInfo{
			Description: "Scripts run in a locked-down ES5.1 sandbox. The only way to affect the outside world is via emit().",
			Blocked:     []string{"require", "fetch", "XMLHttpRequest", "setImmediate", "console.log", "console.warn", "console.error"},
		},
		Examples: []example{
			{
				Title: "Computed prop: live character count",
				Code:  `{"computed": {"char_count": "return 'Characters: ' + ($.value || '').length"}}`,
			},
			{
				Title: "Computed prop: shared state from another node",
				Code:  `{"computed": {"status": "return $('filters').state.activeCount + ' active filters'"}}`,
			},
			{
				Title: "on_change hook: filter a list when input changes",
				Code:  `{"scripts": {"on_change": "emit('agent', {action: 'filter', query: $.value})"}}`,
			},
		},
	}
}

func (s *Server) handleDescribeScripting(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return jsonResult(scriptingCatalog())
}

func (s *Server) handleLayout(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input layoutInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if len(input.Tree) == 0 {
		return errResult("missing required parameter: tree"), nil
	}

	var spec dom.NodeSpec
	if err := json.Unmarshal(input.Tree, &spec); err != nil {
		s.logger.Error("layout: invalid tree spec", "error", err)
		return errResult(fmt.Sprintf("invalid tree spec: %v", err)), nil
	}

	s.logger.Info("layout", "root_id", spec.ID, "root_type", spec.Type, "children", len(spec.Children))

	s.mu.Lock()
	if err := s.tree.ReplaceTree(&spec); err != nil {
		s.mu.Unlock()
		s.logger.Error("layout: ReplaceTree failed", "error", err)
		return errResult(err.Error()), nil
	}
	nodeCount := 0
	s.tree.Walk(func(n *dom.Node) bool { nodeCount++; return true })
	s.mu.Unlock()

	s.logger.Info("layout: success", "node_count", nodeCount)
	s.notifyMutation()
	return jsonResult(okCountResult{OK: true, NodeCount: nodeCount})
}

func (s *Server) handleSetItems(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input setItemsInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if input.Target == "" {
		return errResult("missing required parameter: target"), nil
	}

	var items []map[string]any
	if len(input.Items) > 0 {
		if err := json.Unmarshal(input.Items, &items); err != nil {
			return errResult(fmt.Sprintf("invalid items: %v", err)), nil
		}
	}

	s.logger.Info("set_items", "target", input.Target, "item_count", len(items))

	s.mu.Lock()
	err := s.tree.SetItems(input.Target, items)
	s.mu.Unlock()

	if err != nil {
		s.logger.Error("set_items: failed", "error", err)
		return errResult(err.Error()), nil
	}

	s.logger.Info("set_items: success")
	s.notifyMutation()
	return jsonResult(okCountResult{OK: true, Count: len(items)})
}

func (s *Server) handleAppendItems(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input setItemsInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if input.Target == "" {
		return errResult("missing required parameter: target"), nil
	}

	var items []map[string]any
	if len(input.Items) > 0 {
		if err := json.Unmarshal(input.Items, &items); err != nil {
			return errResult(fmt.Sprintf("invalid items: %v", err)), nil
		}
	}

	s.logger.Info("append_items", "target", input.Target, "item_count", len(items))

	s.mu.Lock()
	err := s.tree.AppendItems(input.Target, items)
	s.mu.Unlock()

	if err != nil {
		s.logger.Error("append_items: failed", "error", err)
		return errResult(err.Error()), nil
	}

	s.logger.Info("append_items: success")
	s.notifyMutation()
	return jsonResult(okCountResult{OK: true, Count: len(items)})
}

func (s *Server) handleRemoveItems(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.IsShutdown() {
		return errResult("server is shutting down"), nil
	}

	var input removeItemsInput
	if err := unmarshalArgs(req, &input); err != nil {
		return errResult(err.Error()), nil
	}
	if input.Target == "" {
		return errResult("missing required parameter: target"), nil
	}

	var keys []string
	if len(input.Keys) > 0 {
		if err := json.Unmarshal(input.Keys, &keys); err != nil {
			return errResult(fmt.Sprintf("invalid keys: %v", err)), nil
		}
	}

	s.logger.Info("remove_items", "target", input.Target, "key_count", len(keys))

	s.mu.Lock()
	err := s.tree.RemoveItems(input.Target, keys)
	s.mu.Unlock()

	if err != nil {
		s.logger.Error("remove_items: failed", "error", err)
		return errResult(err.Error()), nil
	}

	s.logger.Info("remove_items: success")
	s.notifyMutation()
	return jsonResult(okCountResult{OK: true, Removed: len(keys)})
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
