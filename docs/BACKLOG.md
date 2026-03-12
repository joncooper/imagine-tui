# Imagine TUI: development backlog

## TDD strategy

Every milestone has a testing mandate. The rule: **no code merges without corresponding tests, and tests are written first where indicated.**

### Testing layers

1. **DOM unit tests** (pure Go, no terminal, no MCP). The DOM tree, patch operations, snapshot/restore, query, computed prop evaluation, and event routing are all pure data structures and functions. These are tested exhaustively with table-driven tests. This is the largest and most important test surface. TDD is mandatory for this layer — write the test, watch it fail, write the implementation.

2. **Script integration tests** (Go + goja, no terminal). Verify that scripts execute correctly against a DOM, that the $ API works, that computed props re-evaluate, that emit routes correctly. These use a real goja runtime but a mock DOM (or the real DOM from layer 1). TDD is mandatory.

3. **Widget rendering tests** (Go + Lip Gloss, no terminal). Each widget type has golden-file tests: given this node with these props, assert the Lip Gloss output string matches the expected snapshot. Use `lipgloss.NewStyle().Width(80)` to fix terminal width. Golden files are regenerated explicitly, never auto-updated. TDD is encouraged but not mandatory (visual output is iterative).

4. **MCP protocol tests** (Go, no terminal). Verify that the MCP server correctly handles tool calls, returns proper JSON-RPC responses, and handles error cases. Use the MCP SDK's test helpers or a mock transport. TDD is mandatory.

5. **Integration tests** (Go + real BubbleTea test driver). BubbleTea has `teatest` for programmatic input/output. These are end-to-end: send a `replace` tool call via MCP mock, then simulate keypresses via teatest, assert the terminal output and the event returned by `await_event`. These are expensive to write and slow to run — reserve for critical paths only.

6. **Demo smoke tests** (scripted). Each reference demo has a shell script that starts the server, sends a canned sequence of MCP tool calls, and asserts the DOM state via `query`. Not TDD, but required before a milestone is marked complete.

### Coverage targets

- DOM layer: 95%+ line coverage. This is the foundation; bugs here cascade everywhere.
- Script layer: 90%+ on the $ API surface. Every method, every error path.
- Widget rendering: One golden file per widget type per major prop variant.
- MCP protocol: Every tool, every error code, every edge case in the schema.

---

## Milestone 0: Project skeleton & CI

**Goal**: Repo structure, build pipeline, dependency management, and the "hello world" of each layer wired together. Nothing works yet, but everything compiles and the test harness runs.

### M0-1: Repository setup
- Initialize Go module (`github.com/[org]/imagine-tui`)
- Directory structure: `cmd/imagine-tui/`, `internal/dom/`, `internal/script/`, `internal/widget/`, `internal/mcp/`, `internal/render/`, `testdata/`
- Makefile with `build`, `test`, `lint`, `golden-update` targets
- `.golangci-lint.yml` with strict settings
- CI pipeline (GitHub Actions): lint → test → build on every push
- **No TDD here** — this is scaffolding

### M0-2: Dependency lock
- Add dependencies: `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `charmbracelet/bubbles`, `dop251/goja`, MCP Go SDK (evaluate `mark3labs/mcp-go` or `modelcontextprotocol/go-sdk`)
- Verify all compile. Write one trivial test per dependency to confirm import works.
- Document version pins in README

### M0-3: Test harness scaffolding
- Set up `testutil` package with helpers: `NewTestDOM()`, `MustPatch()`, `AssertNodeProps()`, `GoldenFile()`
- Golden file infrastructure: `testdata/golden/` directory, `golden-update` make target, `GOLDEN_UPDATE=1` env var to regenerate
- Confirm `go test ./...` passes with zero tests (no failures on empty)

---

## Milestone 1: The DOM (pure data, no rendering)

**Goal**: A fully functional DOM tree with all CRUD operations, tested exhaustively. This is the foundation everything else builds on. TDD is mandatory for every ticket in this milestone.

### M1-1: Node data structure
- **TDD**: Write tests first for node creation, ID uniqueness, type validation
- Define `Node` struct: ID, Type, Props (map[string]any), Children (ordered slice), Scripts, Computed, parent pointer
- Node constructor validates ID format (non-empty, no whitespace) and type against registry
- Props are typed per widget type — start with a `PropSchema` interface, validate on set
- **Tests**: creation, ID validation, type validation, prop type checking, deep equality

### M1-2: Tree operations
- **TDD**: Write tests for insert, remove, move, reparent, find-by-ID
- `tree.Insert(parentID, node, afterID)` — insert child, optionally after a sibling
- `tree.Remove(id)` — remove node and all descendants, return removed subtree
- `tree.Move(id, newParentID, afterID)` — reparent atomically
- `tree.Find(id)` — O(1) lookup via ID index (map[string]*Node maintained on mutations)
- `tree.Walk(fn)` — depth-first traversal
- `tree.Summary()` — compact string representation (e.g., `"root > header + main(form, table)"`)
- **Tests**: all operations, including error paths (duplicate ID on insert, remove nonexistent, move to descendant of self, move creating cycle). Table-driven, minimum 30 test cases.

### M1-3: Patch engine
- **TDD**: Write tests for each op type, then for multi-op atomicity
- Parse patch ops from JSON: `update`, `insert`, `remove`, `move`
- Apply ops in order, atomically (all succeed or all roll back)
- On `update`: merge props (not replace), merge scripts, merge computed
- Validate all node IDs exist (or will exist after prior ops in the batch)
- Return structured error with op index on failure
- **Tests**: single ops, multi-op batches, rollback on failure, prop merging semantics, ID validation. Minimum 40 test cases.

### M1-4: Replace operation
- **TDD**: Write tests for subtree replacement
- `Replace(id, newTree)` — remove all children of target, insert new subtree
- Validate no ID collisions between new subtree and existing tree (excluding the replaced subtree)
- ID index is rebuilt for the affected subtree
- **Tests**: full replacement, partial replacement, ID collision detection, nested replacement

### M1-5: Query operation
- **TDD**: Write tests for state readback
- `Query(ids []string)` — return props + local state for each requested node
- Include computed values (evaluated at query time)
- Return error for nonexistent IDs (partial success: return what exists, error on missing)
- **Tests**: single node, multiple nodes, nonexistent node, computed prop evaluation at query time

### M1-6: Snapshot & restore
- **TDD**: Write tests for snapshot fidelity and restore correctness
- `Snapshot(name)` — deep copy entire tree + ID index to named store
- `Restore(name)` — replace current tree with deep copy of snapshot, rebuild ID index
- Snapshot store is a `map[string]*TreeSnapshot`
- Deep copy must handle all node fields including scripts (strings) and computed (strings)
- **Tests**: snapshot, restore, restore-then-modify-doesn't-affect-snapshot, overwrite existing snapshot name, restore nonexistent name, multiple snapshots

### M1-7: Event queue
- **TDD**: Write tests for enqueue, dequeue, filtering, debouncing, timeout
- Thread-safe queue (channel or mutex-guarded slice)
- `Enqueue(event)` — add event with source node ID, event type, and auto-collected context
- `Dequeue(opts)` — blocking dequeue with optional timeout, filter, debounce
- Context auto-collection: when enqueueing, walk up to parent, collect sibling values
- `dom_summary` generation from current tree state
- **Tests**: basic enqueue/dequeue, timeout expiry, filter by node ID, debounce coalescing, concurrent enqueue from multiple goroutines, context auto-collection accuracy

---

## Milestone 2: MCP server (protocol layer, no rendering)

**Goal**: A working MCP server that accepts tool calls over stdio, routes them to the DOM layer, and returns proper JSON-RPC responses. Tested against the MCP protocol spec. TDD is mandatory.

### M2-1: MCP server skeleton
- **TDD**: Write protocol-level tests first (send JSON-RPC, assert response shape)
- Initialize MCP server with tool declarations: `patch`, `replace`, `await_event`, `snapshot`, `restore`, `query`
- Tool schemas defined as JSON Schema (these become the contract Claude Code sees)
- Server reads from stdin, writes to stdout (stdio transport)
- **Tests**: server starts, responds to `initialize`, lists tools, handles unknown tool name

### M2-2: Tool: patch
- **TDD**: Write MCP-level tests (JSON-RPC call → response)
- Parse tool input, validate against schema, call DOM patch engine
- Return `{ ok: true }` or structured error
- **Tests**: valid patch, invalid JSON, missing required fields, patch engine error passthrough

### M2-3: Tool: replace
- Same pattern as M2-2 but for replace
- **Tests**: valid replace, ID not found, ID collision in new tree

### M2-4: Tool: await_event
- **TDD**: Write tests for the long-poll lifecycle
- Tool call blocks until event queue has an event (or timeout)
- Parse optional parameters: `timeout_ms`, `filter`, `debounce_ms`
- Return event payload with context and dom_summary
- **Tests**: event fires before call (immediate return), event fires after call (blocked then returns), timeout, filter, debounce, multiple rapid events

### M2-5: Tool: snapshot & restore
- Thin wrappers around DOM snapshot/restore
- **Tests**: round-trip via MCP, error on nonexistent snapshot name

### M2-6: Tool: query
- Thin wrapper around DOM query
- **Tests**: existing nodes, missing nodes, computed prop evaluation via MCP

### M2-7: MCP error handling & edge cases
- Handle malformed JSON-RPC
- Handle tool calls during server shutdown
- Handle concurrent tool calls (should await_event and patch be serialized? Probably yes — document the decision)
- **Tests**: malformed input, concurrent calls, shutdown mid-await

---

## Milestone 3: Script runtime (goja integration)

**Goal**: Claude-authored scripts execute against the DOM, the $ API works, computed props re-evaluate reactively. TDD is mandatory for the $ API surface.

### M3-1: goja sandbox setup
- **TDD**: Write tests that scripts cannot access forbidden APIs
- Initialize goja VM per-session (not per-script — `state` must persist)
- Strip all I/O: no `require`, no `console.log` (provide `debug()` that writes to server log), no `fetch`, no `setTimeout`/`setInterval`
- Confirm sandboxing: test that scripts attempting file/network access fail gracefully
- **Tests**: forbidden API access, script error handling (syntax error, runtime error, infinite loop timeout)

### M3-2: $ API — current node
- **TDD**: Write tests for every property read/write
- Inject `$` as a proxy object bound to the current node
- `$.value`, `$.props`, `$.style`, `$.children`, `$.visible`
- Read operations return current DOM state. Write operations mutate the DOM and trigger re-render.
- **Tests**: read each property, write each property, write triggers dirty flag, write to read-only prop fails

### M3-3: $ API — cross-node access
- **TDD**: Write tests for $('id') access patterns
- `$('id')` returns a proxy for any node in the DOM by ID
- Same property surface as `$` (value, props, style, text, visible, rows)
- Nonexistent ID returns a null-ish object that fails gracefully (no crash, returns undefined on reads)
- **Tests**: access existing node, access nonexistent node, modify cross-node prop, chain access

### M3-4: emit()
- **TDD**: Write tests for both local and claude emit paths
- `emit('local', patchOps)` — apply patch ops to the DOM immediately, synchronously
- `emit('claude', data)` — enqueue event in the event queue for `await_event`
- `emit` with invalid target throws script error
- **Tests**: local emit applies patch, local emit with invalid ops fails, claude emit enqueues, event queue integration

### M3-5: state object
- **TDD**: Write tests for state persistence
- `state` is a plain JS object that persists across script invocations within the same session
- Scoped per-node: each node's scripts share one `state`, but different nodes have isolated state
- `state` survives DOM patches (updating a node's scripts doesn't reset its state)
- **Tests**: state persists across invocations, state isolation between nodes, state survives prop updates, state does not survive node removal

### M3-6: Script lifecycle hooks
- **TDD**: Write tests for each hook trigger condition
- Wire hooks to DOM events: `on_mount` fires after insert, `on_change` fires after value mutation, `on_event` fires on child events (bubbling), `on_focus`/`on_blur`, `on_key`
- Scripts execute synchronously (block the frame loop briefly — enforce a timeout, e.g., 10ms)
- **Tests**: each hook fires at the right time, hook receives correct event payload, timeout kills runaway scripts

### M3-7: Computed props
- **TDD**: Write tests for dependency tracking and re-evaluation
- Computed props are script strings that return a value
- Runtime tracks which nodes a computed prop reads (via $('id') calls during evaluation)
- When a dependency changes, re-evaluate the computed prop and update the node
- Cycle detection: if A computes from B and B computes from A, error and break the cycle
- **Tests**: basic computation, dependency tracking, re-evaluation on change, cycle detection, error in computed prop expression

---

## Milestone 4: Widget library (v1 core set)

**Goal**: All v1 widget types render correctly to the terminal via Lip Gloss. Golden-file tests for every widget. TDD encouraged for behavior; golden files for visual output.

### M4-1: Widget registry & rendering pipeline
- Define `WidgetRenderer` interface: `Render(node *Node, width int, height int) string`
- Widget registry: `map[NodeType]WidgetRenderer`
- Rendering pipeline: walk DOM tree, call renderer for each node, compose output
- Handle terminal resize (re-render with new dimensions)
- **Tests**: registry lookup, unknown type error, basic composition

### M4-2: container widget
- Flex-like layout: `direction` (horizontal/vertical), `gap`, `padding`, `border`
- Child width distribution: fixed (chars), percentage, `fill` (take remaining space)
- Focus cycling: Tab/Shift-Tab moves focus among focusable children
- **Golden files**: vertical layout, horizontal layout, nested containers, border variants, resize behavior

### M4-3: text widget
- Styled text with Lip Gloss format tokens
- Props: `text` (content), `style` (token string like `"bold danger"`)
- Handle word wrap, truncation with ellipsis
- Inline style spans (for mixed-style text like annotations): `segments` prop as array of `{text, style}` pairs
- **Golden files**: plain text, styled text, wrapped text, truncated text, mixed segments

### M4-4: input widget
- Back by Bubbles textarea (single-line mode)
- Props: `placeholder`, `value`, `pattern` (regex for auto-validation), `style`
- Emits: `change` (on every keystroke), `submit` (on Enter)
- Visual validation feedback: border color changes based on pattern match
- **Golden files**: empty with placeholder, with value, focused, invalid state
- **Behavior tests**: keystroke updates value, Enter emits submit, pattern validation

### M4-5: textarea widget
- Multi-line variant of input
- Props: `placeholder`, `value`, `max_lines`, `style`
- Scrollable when content exceeds max_lines
- **Golden files**: empty, with content, scrolled, focused

### M4-6: select widget
- Backed by Bubbles list (single-select mode) or custom multi-select
- Props: `options` (array of `{label, value}`), `selected`, `multi`, `filterable`
- Default behavior: arrow keys navigate, type to filter (when `filterable: true`), Enter selects
- Emits: `change` (on selection change)
- **Golden files**: closed, open, with filter text, multi-select with checkmarks
- **Behavior tests**: navigation, filtering, selection, multi-select toggle

### M4-7: button widget
- Focusable, styled, triggers events
- Props: `label`, `style`, `disabled`
- Emits: `click` (on Enter or Space when focused)
- Visual states: default, focused, disabled, active (pressed)
- **Golden files**: default, focused, disabled, each style variant
- **Behavior tests**: click on Enter, click on Space, no click when disabled

### M4-8: table widget
- Rows + columns, sortable, scrollable, expandable
- Props: `columns` (array of `{key, label, width, sortable}`), `rows` (array of objects), `expandable`, `row_style` (script or mapping for per-row styling)
- Default behavior: arrow keys scroll, Enter expands/collapses row (if expandable), column header click sorts (if sortable)
- Emits: `select` (row selected), `sort` (column sort triggered), `expand` (row expanded)
- Row status styling: `row_style` maps row data to style tokens (e.g., `"success"` for passed tests)
- **Golden files**: basic table, sorted column (with indicator), expanded row, row status colors, scrolled position, empty state
- **Behavior tests**: sort toggle, row selection, row expansion, scroll bounds

### M4-9: list widget
- Vertical item list with selection and badges
- Props: `items` (array of `{id, label, badge, style}`), `selected`, `filterable`
- Default behavior: arrow keys navigate, type to filter, Enter selects
- Emits: `select` (on item selection)
- **Golden files**: basic list, with badges, with filter active, selected item, empty state

### M4-10: diff widget
- Split or unified diff view
- Props: `hunks` (array of `{old_start, new_start, lines}`), `mode` ("split" | "unified"), `file_name`
- Each line: `{type: "add"|"remove"|"context", content, old_num, new_num}`
- Default behavior: `d` toggles split/unified, `n`/`p` for next/prev hunk, arrow keys scroll, line numbers displayed
- Emits: `select_line` (line clicked/entered), `hunk_navigate` (hunk change)
- **Golden files**: unified mode, split mode, add-heavy hunk, remove-heavy hunk, context lines, file header
- **Behavior tests**: mode toggle, hunk navigation, line selection

### M4-11: log widget
- Append-only scrolling text
- Props: `lines` (array of `{text, level, timestamp}`), `auto_scroll`, `max_lines`
- ANSI passthrough: log lines can contain raw ANSI escape codes (from test runners, build tools)
- Sticky-bottom: auto-scrolls unless user scrolled up manually. Scrolling back down re-engages auto-scroll.
- Level coloring: `error` → red, `warn` → yellow, `info` → default, `debug` → muted
- Append semantics: `patch` op `update` on a log node with `append_lines` prop adds lines without replacing existing ones
- **Golden files**: basic log, mixed severity levels, ANSI passthrough, scrolled-up state
- **Behavior tests**: append, auto-scroll, sticky-bottom re-engage, max_lines truncation

### M4-12: code widget
- Syntax-highlighted code block
- Props: `content`, `language`, `line_numbers` (bool), `highlight_lines` (array of line numbers or ranges), `start_line` (offset for line numbering)
- Highlight uses a Go syntax highlighter (e.g., Chroma) mapped to Lip Gloss styles
- Line range highlighting: specified lines get a background accent
- Default behavior: arrow keys scroll, `y` yanks highlighted range to clipboard (via OSC 52)
- **Golden files**: Go code, Python code, with highlighted lines, with line number offset
- **Behavior tests**: scroll, yank to clipboard

---

## Milestone 5: BubbleTea integration

**Goal**: The DOM renders live in a terminal via BubbleTea. The MCP server, DOM, scripts, and widgets are all wired together into a running application.

### M5-1: BubbleTea model & update loop
- Define the top-level BubbleTea model: holds DOM tree, script runtime, event queue, MCP connection state
- Update loop: process terminal events (keypresses, resize), route to focused widget, trigger scripts, queue Claude-routed events
- View function: walk DOM, call widget renderers, compose final output string
- **Tests (teatest)**: basic render, keypress routing to focused widget, resize re-renders

### M5-2: Focus management
- Focus ring: ordered list of focusable node IDs (derived from DOM walk)
- Tab/Shift-Tab cycles focus
- Container focus trapping (modal behavior)
- Focus state reflected in widget rendering (border highlight, cursor visibility)
- **Tests**: focus cycle order, Tab wraps, Shift-Tab wraps, container trapping, focus after node removal

### M5-3: MCP server ↔ BubbleTea bridge
- MCP tool calls arrive on a goroutine, DOM mutations need to reach the BubbleTea Update loop
- Use BubbleTea's `tea.Program.Send()` to inject custom messages from the MCP goroutine
- Serialize DOM mutations: MCP goroutine acquires a lock, applies patch, signals BubbleTea to re-render
- `await_event` blocks the MCP goroutine (not the BubbleTea loop) via the event queue channel
- **Tests**: patch from MCP triggers re-render, await_event blocks correctly, concurrent patch + keypress

### M5-4: Terminal resize handling
- On `tea.WindowSizeMsg`, re-render all widgets with new dimensions
- Container layout recalculates child sizes
- Percentage-width children recompute
- **Tests (teatest)**: resize shrinks layout, resize grows layout, nested container resize

### M5-5: Startup & shutdown
- `imagine-tui serve` starts MCP server on stdio and BubbleTea on the terminal
- Handle Ctrl+C: graceful shutdown, drain event queue, close MCP connection
- Handle broken pipe (Claude Code disconnects): show "disconnected" in TUI, wait for reconnect or quit
- **Tests**: startup completes, Ctrl+C shuts down cleanly, broken pipe shows message

---

## Milestone 6: End-to-end demo — test whisperer

**Goal**: The first reference demo runs end-to-end. This validates the full stack: MCP → DOM → scripts → widgets → terminal. It also serves as the integration test suite for the entire system.

### M6-1: Demo script & system prompt
- Write the Claude Code system prompt / CLAUDE.md instructions for the test whisperer demo
- Define the MCP tool call sequence: initial `replace` with table + status bar, `await_event` loop
- Write a mock test runner that produces canned failures for deterministic testing
- Document the expected interaction flow

### M6-2: Initial screen build
- Claude Code calls `replace` with: header (text), failure table (table with row_style), status bar (text with computed summary)
- Verify the full render pipeline: MCP → DOM → widgets → terminal
- **Smoke test**: canned replace payload renders correctly

### M6-3: Row interaction
- User selects a row, presses Enter → event routed to Claude
- Claude reads test file + source file (via filesystem MCP server)
- Claude patches in a detail panel (code widget + text annotation + button)
- **Smoke test**: canned event → canned patch → correct render

### M6-4: Fix-it flow
- User clicks "Fix it" → event to Claude
- Claude writes file (via filesystem), runs test (via shell), patches row status
- Row transitions from red to green with animation (style change)
- **Smoke test**: full cycle with mock filesystem and shell

### M6-5: Local scripts
- Table sort/filter scripts (goja)
- Computed summary: "3 of 14 fixed" updates automatically as rows change status
- Status bar shows elapsed time (computed from `on_tick`)
- **Tests**: sort script, filter script, computed summary accuracy

### M6-6: Demo polish & recording
- Record a terminal session (asciinema or similar) for the README
- Write setup instructions: how to configure Claude Code MCP, run the server, trigger the demo
- Performance profiling: measure latency at each tier

---

## Milestone 7: Extended demos & hardening

**Goal**: Ship 2-3 more reference demos, harden edge cases, prepare for external users.

### M7-1: Living dashboard demo
- Exercises multi-MCP-server orchestration, sparkline (may need v2 widget), container layout
- Requires sparkline widget implementation (promote from v2 if not already done)

### M7-2: Log detective demo
- Exercises log widget, cross-file reasoning, timeline visualization
- Most complex multi-tool orchestration: log parsing + source file reading + causal tracing

### M7-3: TSX fragment runtime
- Implement the JSX transform in the goja layer
- Component registry: `<ListItem>`, `<Text>`, `<Container>` → DOM node specs
- Compile step: string transform before passing to goja, or runtime tagged template processing
- **TDD**: transform input/output pairs, component registry lookup, fragment rendering

### M7-4: Session persistence
- Serialize DOM + snapshot store + script state to JSON on disk
- Resume from serialized state on startup
- **TDD**: serialize/deserialize round-trip, resume renders correctly

### M7-5: Error recovery & resilience
- Script timeout handling (goja interrupt)
- Malformed patch recovery (error message shown in TUI, not crash)
- MCP reconnection after disconnect
- DOM consistency checks (orphaned nodes, dangling references)
- **TDD**: every error path, every recovery mechanism

### M7-6: Documentation & packaging
- README with architecture overview, quickstart, demo walkthroughs
- MCP tool schema documentation (auto-generated from Go types)
- Go binary releases for macOS (arm64, amd64), Linux (amd64)
- Homebrew formula or `go install` instructions

---

## Milestone ordering & dependencies

```
M0 (skeleton)
 └→ M1 (DOM)           ← foundation, must be rock-solid
     ├→ M2 (MCP)       ← protocol layer atop DOM
     └→ M3 (scripts)   ← scripting atop DOM
         └→ M4 (widgets) ← rendering atop DOM + scripts
              └→ M5 (BubbleTea integration) ← wires everything together
                   └→ M6 (test whisperer demo) ← first end-to-end
                        └→ M7 (extended demos & hardening)
```

M1 is the critical path. M2 and M3 can proceed in parallel once M1 is complete. M4 depends on M3 (widgets need to trigger scripts). M5 depends on M2 + M4 (wires MCP and widgets into BubbleTea). M6 is the integration proof. M7 is iterative.

## Effort estimates

| Milestone | Tickets | Estimated effort | Risk |
|-----------|---------|-----------------|------|
| M0: Skeleton | 3 | 1–2 days | Low |
| M1: DOM | 7 | 2–3 weeks | Medium (design decisions in patch semantics) |
| M2: MCP | 7 | 1–2 weeks | Low (protocol is well-defined) |
| M3: Scripts | 7 | 2–3 weeks | High (goja edge cases, sandboxing) |
| M4: Widgets | 12 | 3–4 weeks | Medium (visual polish is iterative) |
| M5: Integration | 5 | 2–3 weeks | High (concurrency, BubbleTea bridge) |
| M6: Test whisperer | 6 | 1–2 weeks | Medium (first e2e, expect surprises) |
| M7: Extended | 6 | 3–4 weeks | Medium |

**Total: ~16–22 weeks for one developer, ~10–14 weeks for two working in parallel on M2/M3 after M1.**