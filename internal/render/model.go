package render

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/script"
	"github.com/joncooper/imagine-tui/internal/telemetry"
	"github.com/joncooper/imagine-tui/internal/widget"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

// Model is the top-level BubbleTea model that wires together the DOM tree,
// widget renderers, event queue, and MCP server.
type Model struct {
	server   *imcp.Server
	widgets  *widget.Tree
	focus    *FocusRing
	logger   *slog.Logger
	tracer   trace.Tracer
	traceCtx context.Context

	focusedID           string
	width               int
	height              int
	activeSessionID     uint64
	waitingForReconnect bool
	scripts             *script.Runtime
	scriptTree          *dom.Tree
	knownNodes          map[string]bool
	timerSeq            uint64
}

// NewModel creates a new render Model backed by the given MCP server and widget registry.
func NewModel(srv *imcp.Server, registry *widget.Registry) Model {
	return Model{
		server:     srv,
		widgets:    widget.NewTree(registry),
		focus:      &FocusRing{index: make(map[string]int)},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		tracer:     nooptrace.NewTracerProvider().Tracer("github.com/joncooper/imagine-tui/internal/render"),
		traceCtx:   context.Background(),
		knownNodes: make(map[string]bool),
	}
}

// SetLogger sets the structured logger for the render model and its widget tree.
func (m *Model) SetLogger(l *slog.Logger) {
	m.logger = l
	m.widgets.SetLogger(l)
}

// SetTracer sets the tracer used for render instrumentation.
func (m *Model) SetTracer(t trace.Tracer) {
	m.tracer = t
}

// Init implements tea.Model. No startup command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Routes messages to the appropriate handler.
// Acquires the server's write lock for the duration of the update to serialize
// with concurrent MCP handler goroutines that also mutate the DOM under Lock.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	ctx := m.contextForMsg(msg)
	ctx, span := m.tracer.Start(
		ctx,
		"render.update",
		trace.WithAttributes(attribute.String("render.message_type", messageType(msg))),
	)
	defer span.End()

	m.server.Lock()
	defer m.server.Unlock()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.logger.InfoContext(ctx, "window resize", "width", msg.Width, "height", msg.Height)
		m.width = msg.Width
		m.height = msg.Height
		cmd := m.withTimerCmd(m.syncState(ctx))
		return m, cmd

	case tea.KeyMsg:
		span.SetAttributes(
			attribute.String("render.key_type", msg.Type.String()),
			attribute.String("render.key_runes", string(msg.Runes)),
		)
		m.logger.InfoContext(ctx, "key", "type", msg.Type, "runes", string(msg.Runes),
			"focused", m.focusedID, "waiting_for_reconnect", m.waitingForReconnect)
		// Allow 'q' or Esc to quit when disconnected or no DOM loaded.
		if m.waitingForReconnect || m.server.Tree() == nil {
			if msg.Type == tea.KeyRunes && string(msg.Runes) == "q" || msg.Type == tea.KeyEsc {
				m.server.Shutdown()
				return m, tea.Quit
			}
		}
		next, cmd := m.handleKey(ctx, msg)
		return next, cmd

	case DOMChangedMsg:
		m.traceCtx = ctx
		m.logger.InfoContext(ctx, "DOM changed, syncing state")
		cmd, err := m.refreshScriptsAndWidgets(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script sync failed", "error", err)
			cmd = m.syncState(ctx)
		}
		// If focused node was removed, adjust focus.
		if m.focusedID != "" && !m.focus.Contains(m.focusedID) {
			m.focusedID = m.focus.Next("")
			m.logger.InfoContext(ctx, "focus adjusted (removed node)", "new_focus", m.focusedID)
		}
		// Auto-focus first element if nothing is focused.
		if m.focusedID == "" && len(m.focus.IDs) > 0 {
			m.focusedID = m.focus.IDs[0]
			m.logger.InfoContext(ctx, "auto-focus first element", "focused", m.focusedID)
		}
		cmd = m.withTimerCmd(cmd)
		return m, cmd

	case MCPDisconnectedMsg:
		m.traceCtx = context.Background()
		if msg.SessionID != 0 && msg.SessionID != m.activeSessionID {
			m.logger.InfoContext(ctx, "ignoring stale MCP disconnect", "session_id", msg.SessionID, "active_session_id", m.activeSessionID, "error", msg.Err)
			return m, nil
		}
		m.logger.InfoContext(ctx, "MCP disconnected", "session_id", msg.SessionID, "error", msg.Err)
		m.activeSessionID = 0
		m.waitingForReconnect = true
		cmd := m.withTimerCmd(nil)
		return m, cmd

	case MCPConnectedMsg:
		m.traceCtx = ctx
		m.logger.InfoContext(ctx, "MCP connected", "session_id", msg.SessionID)
		m.activeSessionID = msg.SessionID
		m.waitingForReconnect = false
		cmd, err := m.refreshScriptsAndWidgets(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script sync failed", "error", err)
			cmd = m.syncState(ctx)
		}
		cmd = m.withTimerCmd(cmd)
		return m, cmd

	case widget.CommandMsg:
		return m.routeWidgetMessage(ctx, msg)

	case ShutdownMsg:
		m.server.Shutdown()
		return m, tea.Quit

	case scriptTimerMsg:
		m.traceCtx = ctx
		if msg.Seq != m.timerSeq || m.scripts == nil {
			return m, nil
		}
		ran, err := m.scripts.RunDueTimers(msg.FiredAt)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script timer failed", "error", err)
		}
		if ran {
			if _, err := m.refreshScriptsAndWidgets(ctx); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				m.logger.ErrorContext(ctx, "script refresh failed", "error", err)
				m.syncState(ctx)
			}
		}
		cmd := m.withTimerCmd(nil)
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model. Renders the DOM tree via the widget tree.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		m.logger.Debug("view: zero dimensions", "width", m.width, "height", m.height)
		return ""
	}

	m.server.RLock()
	defer m.server.RUnlock()
	tree := m.server.Tree()
	m.logger.Debug("view: rendering", "root_children", len(tree.Root.Children),
		"width", m.width, "height", m.height)
	var result string
	if len(tree.Root.Children) == 0 {
		result = m.renderIdleScreen()
	} else {
		result = m.widgets.Render(tree, m.width, m.height, m.focusedID)
	}
	m.logger.Debug("view: done", "output_len", len(result))
	if m.waitingForReconnect {
		return m.renderReconnectBanner(result)
	}
	return result
}

// syncState synchronizes the widget tree and focus ring with the current DOM.
// Callers must hold the server's Lock or RLock.
func (m *Model) syncState(ctx context.Context) tea.Cmd {
	ctx, span := m.tracer.Start(ctx, "render.sync_state")
	defer span.End()

	tree := m.server.Tree()
	m.logger.DebugContext(ctx, "syncState", "root_children", len(tree.Root.Children),
		"tree_summary", tree.Summary())
	if err := m.widgets.Sync(tree); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.logger.ErrorContext(ctx, "syncState: widget sync failed", "error", err)
	}
	m.focus = BuildFocusRing(tree)
	span.SetAttributes(
		attribute.Int("render.root_children", len(tree.Root.Children)),
		attribute.Int("render.focusable_count", len(m.focus.IDs)),
	)
	m.logger.DebugContext(ctx, "focus ring built", "size", len(m.focus.IDs), "ids", m.focus.IDs)
	cmds := m.widgets.Commands(tree)
	span.SetAttributes(attribute.Int("render.widget_command_count", len(cmds)))
	return batchCmds(cmds...)
}

// handleKey processes keyboard input. Must be called under the server's Lock
// (held by Update).
func (m Model) handleKey(ctx context.Context, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.server.Shutdown()
		return m, tea.Quit

	case tea.KeyTab:
		oldFocus := m.focusedID
		m.focusedID = m.focusTab(true)
		m.logger.InfoContext(ctx, "tab focus", "new_focus", m.focusedID)
		cmd := m.withTimerCmd(m.runFocusHooks(ctx, oldFocus, m.focusedID))
		return m, cmd

	case tea.KeyShiftTab:
		oldFocus := m.focusedID
		m.focusedID = m.focusTab(false)
		m.logger.InfoContext(ctx, "shift-tab focus", "new_focus", m.focusedID)
		cmd := m.withTimerCmd(m.runFocusHooks(ctx, oldFocus, m.focusedID))
		return m, cmd

	default:
		// Route to focused widget.
		if m.focusedID != "" {
			next, cmd := m.routeKeyToWidget(ctx, msg)
			timerCmd := next.withTimerCmd(cmd)
			return next, timerCmd
		}
		cmd := m.withTimerCmd(nil)
		return m, cmd
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
func (m Model) routeKeyToWidget(ctx context.Context, msg tea.KeyMsg) (Model, tea.Cmd) {
	ctx, span := m.tracer.Start(
		ctx,
		"render.route_key_to_widget",
		trace.WithAttributes(
			attribute.String("render.focused_id", m.focusedID),
			attribute.String("render.key_type", msg.Type.String()),
		),
	)
	defer span.End()

	w := m.widgets.Get(m.focusedID)
	if w == nil {
		return m, nil
	}

	node := m.server.Tree().Find(m.focusedID)
	if node == nil {
		return m, nil
	}

	keyHookRan, hookCmd := m.runKeyHook(ctx, node.ID, msg)
	if keyHookRan {
		node = m.server.Tree().Find(m.focusedID)
		if node == nil {
			return m, hookCmd
		}
		if node.Type == dom.TypeButton && (msg.Type == tea.KeyEnter || msg.Type == tea.KeySpace) {
			return m, hookCmd
		}
		w = m.widgets.Get(m.focusedID)
		if w == nil {
			return m, hookCmd
		}
	}

	result := w.Update(msg, node)

	eventCmd := m.routeWidgetEvents(ctx, result.Events)
	return m, batchCmds(hookCmd, widget.WrapCmd(node.ID, result.Cmd), eventCmd)
}

func (m Model) routeWidgetMessage(ctx context.Context, msg widget.CommandMsg) (tea.Model, tea.Cmd) {
	ctx, span := m.tracer.Start(
		ctx,
		"render.route_widget_message",
		trace.WithAttributes(attribute.String(telemetry.AttrWidgetNodeID, msg.NodeID)),
	)
	defer span.End()

	w := m.widgets.Get(msg.NodeID)
	if w == nil {
		return m, nil
	}

	node := m.server.Tree().Find(msg.NodeID)
	if node == nil {
		return m, nil
	}

	result := w.Update(msg.Msg, node)
	eventCmd := m.routeWidgetEvents(ctx, result.Events)
	return m, batchCmds(widget.WrapCmd(node.ID, result.Cmd), eventCmd)
}

func (m *Model) withTimerCmd(cmd tea.Cmd) tea.Cmd {
	timerCmd := m.scheduleTimerCmd()
	switch {
	case cmd == nil:
		return timerCmd
	case timerCmd == nil:
		return cmd
	default:
		return tea.Batch(cmd, timerCmd)
	}
}

func (m *Model) scheduleTimerCmd() tea.Cmd {
	m.timerSeq++
	seq := m.timerSeq

	if m.scripts == nil {
		return nil
	}

	nextAt, ok := m.scripts.NextTimerAt()
	if !ok {
		return nil
	}

	delay := time.Until(nextAt)
	if delay < 0 {
		delay = 0
	}

	timerCtx := m.traceCtx
	return tea.Tick(delay, func(firedAt time.Time) tea.Msg {
		return scriptTimerMsg{
			Seq:     seq,
			FiredAt: firedAt,
			Ctx:     timerCtx,
		}
	})
}

// routeWidgetEvent routes a widget event to the appropriate destination:
// scripts first, then if the event should be agent-routed, enqueue it.
func (m *Model) routeWidgetEvents(ctx context.Context, events []widget.Event) tea.Cmd {
	var cmds []tea.Cmd
	for _, evt := range events {
		cmds = append(cmds, m.routeWidgetEvent(ctx, evt))
	}
	return batchCmds(cmds...)
}

// routeWidgetEvent routes a widget event to the appropriate destination:
// scripts first, then if the event should be agent-routed, enqueue it.
func (m *Model) routeWidgetEvent(ctx context.Context, evt widget.Event) tea.Cmd {
	ctx, span := m.tracer.Start(
		ctx,
		"render.route_widget_event",
		trace.WithAttributes(
			attribute.String(telemetry.AttrWidgetEvent, evt.Type),
			attribute.String(telemetry.AttrWidgetNodeID, evt.NodeID),
		),
	)
	defer span.End()

	node := m.server.Tree().Find(evt.NodeID)
	if node == nil {
		return nil
	}

	refreshNeeded := false
	handled := false
	var refreshCmd tea.Cmd

	switch evt.Type {
	case "change":
		if selected, ok := evt.Data["selected"]; ok {
			node.SetProp("value", selected)
		}
		refreshNeeded = true
		if m.hasHook(evt.NodeID, script.HookOnChange) {
			refreshNeeded = true
			handled = true
			value, ok := evt.Data["value"]
			if !ok {
				value = evt.Data["selected"]
			}
			if err := m.scripts.NotifyChange(evt.NodeID, value); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				m.logger.ErrorContext(ctx, "script on_change failed", "node_id", evt.NodeID, "error", err)
			}
		}
	case "submit":
		if m.hasHook(evt.NodeID, script.HookOnSubmit) {
			refreshNeeded = true
			payload := &script.HookPayload{Data: evt.Data}
			if _, err := m.scripts.ExecHook(evt.NodeID, script.HookOnSubmit, payload); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				m.logger.ErrorContext(ctx, "script on_submit failed", "node_id", evt.NodeID, "error", err)
			}
		}
	}

	if m.runEventHooks(ctx, node, evt) {
		refreshNeeded = true
	}
	if refreshNeeded {
		cmd, err := m.refreshScriptsAndWidgets(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script refresh failed", "source", evt.NodeID, "error", err)
		} else {
			refreshCmd = cmd
		}
	}
	if handled {
		return refreshCmd
	}

	// Enqueue as an agent-routed event.
	domEvt := &dom.Event{
		Type:   evt.Type,
		Source: evt.NodeID,
		Data:   evt.Data,
	}

	// Auto-collect context from siblings.
	domEvt.Context = m.collectContext(evt.NodeID)
	domEvt.DOMSummary = m.server.Tree().Summary()

	m.server.Events().Enqueue(domEvt)
	span.SetAttributes(attribute.Bool("render.agent_routed", true))
	return refreshCmd
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

func (m *Model) ensureScriptRuntime() {
	tree := m.server.Tree()
	if tree == nil {
		return
	}
	if m.scripts != nil && m.scriptTree == tree {
		return
	}

	m.scriptTree = tree
	m.scripts = script.New(tree, m.server.Events())
	m.knownNodes = make(map[string]bool)
}

func (m *Model) refreshScriptsAndWidgets(ctx context.Context) (tea.Cmd, error) {
	ctx, span := m.tracer.Start(ctx, "render.refresh_scripts_and_widgets")
	defer span.End()

	m.ensureScriptRuntime()
	if m.scripts != nil {
		if err := m.reconcileScriptTree(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		if err := m.scripts.EvalAllComputed(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	}
	return m.syncState(ctx), nil
}

func (m *Model) reconcileScriptTree() error {
	const maxIterations = 32
	for i := 0; i < maxIterations; i++ {
		order, live := m.collectNodeOrder()
		for id := range m.knownNodes {
			if !live[id] {
				m.scripts.NotifyRemove(id)
				delete(m.knownNodes, id)
			}
		}

		var added []string
		for _, id := range order {
			if !m.knownNodes[id] {
				added = append(added, id)
			}
		}
		if len(added) == 0 {
			return nil
		}

		for _, id := range added {
			m.knownNodes[id] = true
			if err := m.scripts.NotifyMount(id); err != nil {
				return err
			}
		}
	}

	return &script.Error{Hook: "on_mount", Message: "mount reconciliation exceeded iteration limit"}
}

func (m *Model) collectNodeOrder() (order []string, live map[string]bool) {
	tree := m.server.Tree()
	if tree == nil {
		return nil, nil
	}

	live = make(map[string]bool)
	tree.Walk(func(n *dom.Node) bool {
		order = append(order, n.ID)
		live[n.ID] = true
		return true
	})
	return
}

func (m *Model) hasHook(nodeID string, hook script.HookType) bool {
	m.ensureScriptRuntime()
	node := m.server.Tree().Find(nodeID)
	if node == nil {
		return false
	}
	body, ok := node.Scripts[string(hook)]
	return ok && body != ""
}

func (m *Model) runKeyHook(ctx context.Context, nodeID string, msg tea.KeyMsg) (bool, tea.Cmd) {
	if !m.hasHook(nodeID, script.HookOnKey) {
		return false, nil
	}
	if _, err := m.scripts.ExecHook(nodeID, script.HookOnKey, &script.HookPayload{Key: msg.String()}); err != nil {
		trace.SpanFromContext(ctx).RecordError(err)
		trace.SpanFromContext(ctx).SetStatus(codes.Error, err.Error())
		m.logger.ErrorContext(ctx, "script on_key failed", "node_id", nodeID, "error", err)
	}
	cmd, err := m.refreshScriptsAndWidgets(ctx)
	if err != nil {
		trace.SpanFromContext(ctx).RecordError(err)
		trace.SpanFromContext(ctx).SetStatus(codes.Error, err.Error())
		m.logger.ErrorContext(ctx, "script refresh failed", "node_id", nodeID, "error", err)
	}
	return true, cmd
}

func (m *Model) runFocusHooks(ctx context.Context, oldFocus, newFocus string) tea.Cmd {
	ran := false
	if oldFocus != "" && m.hasHook(oldFocus, script.HookOnBlur) {
		if _, err := m.scripts.ExecHook(oldFocus, script.HookOnBlur, nil); err != nil {
			trace.SpanFromContext(ctx).RecordError(err)
			trace.SpanFromContext(ctx).SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script on_blur failed", "node_id", oldFocus, "error", err)
		}
		ran = true
	}
	if newFocus != "" && m.hasHook(newFocus, script.HookOnFocus) {
		if _, err := m.scripts.ExecHook(newFocus, script.HookOnFocus, nil); err != nil {
			trace.SpanFromContext(ctx).RecordError(err)
			trace.SpanFromContext(ctx).SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script on_focus failed", "node_id", newFocus, "error", err)
		}
		ran = true
	}
	if ran {
		cmd, err := m.refreshScriptsAndWidgets(ctx)
		if err != nil {
			trace.SpanFromContext(ctx).RecordError(err)
			trace.SpanFromContext(ctx).SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script refresh failed", "old_focus", oldFocus, "new_focus", newFocus, "error", err)
			return nil
		}
		return cmd
	}
	return nil
}

func (m *Model) runEventHooks(ctx context.Context, node *dom.Node, evt widget.Event) bool {
	if m.scripts == nil {
		m.ensureScriptRuntime()
	}
	if m.scripts == nil || node == nil {
		return false
	}

	ran := false
	payload := &script.HookPayload{
		Event:  evt.Type,
		Source: evt.NodeID,
		Data:   evt.Data,
	}
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		body, ok := parent.Scripts[string(script.HookOnEvent)]
		if !ok || body == "" {
			continue
		}
		if _, err := m.scripts.ExecHook(parent.ID, script.HookOnEvent, payload); err != nil {
			trace.SpanFromContext(ctx).RecordError(err)
			trace.SpanFromContext(ctx).SetStatus(codes.Error, err.Error())
			m.logger.ErrorContext(ctx, "script on_event failed", "node_id", parent.ID, "source", evt.NodeID, "error", err)
		}
		ran = true
	}
	return ran
}

func (m Model) renderReconnectBanner(content string) string {
	return m.renderModalOverlay(content, "Imagine TUI", "Agent disconnected.\nWaiting for reconnect...")
}

func (m Model) renderIdleScreen() string {
	status := "Waiting for agent..."
	if m.activeSessionID != 0 && !m.waitingForReconnect {
		status = "Agent connected."
	}

	return m.renderStatusScreen("Imagine TUI", status)
}

func (m Model) renderStatusScreen(title, status string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Align(lipgloss.Center)
	bodyStyle := lipgloss.NewStyle().Faint(true).Align(lipgloss.Center)
	boxStyle := statusBoxStyle(title, status)

	box := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(title),
		bodyStyle.Render(status),
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(box),
	)
}

func (m Model) renderModalOverlay(content, title, status string) string {
	boxStyle := statusBoxStyle(title, status)
	titleStyle := lipgloss.NewStyle().Bold(true).Align(lipgloss.Center)
	bodyStyle := lipgloss.NewStyle().Faint(true).Align(lipgloss.Center)

	box := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(title),
		bodyStyle.Render(status),
	)
	baseLines := strings.Split(content, "\n")
	for len(baseLines) < m.height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > m.height {
		baseLines = baseLines[:m.height]
	}
	for i := range baseLines {
		baseLines[i] = padOverlayLine(baseLines[i], m.width)
	}

	modalLines := strings.Split(boxStyle.Render(box), "\n")
	startRow := (m.height - len(modalLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, modalLine := range modalLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = overlayLine(baseLines[row], modalLine, m.width)
	}

	return strings.Join(baseLines, "\n")
}

func padOverlayLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	if pad := width - lipgloss.Width(line); pad > 0 {
		return line + strings.Repeat(" ", pad)
	}
	return line
}

func overlayLine(base, overlay string, width int) string {
	if width <= 0 {
		return base
	}

	baseRunes := []rune(padOverlayLine(base, width))
	if len(baseRunes) < width {
		baseRunes = append(baseRunes, []rune(strings.Repeat(" ", width-len(baseRunes)))...)
	}

	overlayRunes := []rune(overlay)
	startCol := (width - lipgloss.Width(overlay)) / 2
	if startCol < 0 {
		startCol = 0
	}
	if startCol >= len(baseRunes) {
		return string(baseRunes)
	}
	if avail := len(baseRunes) - startCol; len(overlayRunes) > avail {
		overlayRunes = overlayRunes[:avail]
	}
	copy(baseRunes[startCol:startCol+len(overlayRunes)], overlayRunes)
	return string(baseRunes)
}

func statusBoxStyle(title, status string) lipgloss.Style {
	width := 28
	for _, line := range append([]string{title}, strings.Split(status, "\n")...) {
		lineWidth := lipgloss.Width(line) + 6
		if lineWidth > width {
			width = lineWidth
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 3).
		Width(width).
		Align(lipgloss.Center)
}

func (m Model) contextForMsg(msg tea.Msg) context.Context {
	switch msg := msg.(type) {
	case DOMChangedMsg:
		if msg.Ctx != nil {
			return msg.Ctx
		}
	case MCPConnectedMsg:
		if msg.Ctx != nil {
			return msg.Ctx
		}
	case MCPDisconnectedMsg:
		if msg.Ctx != nil {
			return msg.Ctx
		}
	case scriptTimerMsg:
		if msg.Ctx != nil {
			return msg.Ctx
		}
	}
	if m.traceCtx != nil {
		return m.traceCtx
	}
	return context.Background()
}

func messageType(msg tea.Msg) string {
	switch msg.(type) {
	case tea.WindowSizeMsg:
		return "window_size"
	case tea.KeyMsg:
		return "key"
	case DOMChangedMsg:
		return "dom_changed"
	case MCPConnectedMsg:
		return "mcp_connected"
	case MCPDisconnectedMsg:
		return "mcp_disconnected"
	case widget.CommandMsg:
		return "widget_command"
	case ShutdownMsg:
		return "shutdown"
	case scriptTimerMsg:
		return "script_timer"
	default:
		return "unknown"
	}
}
