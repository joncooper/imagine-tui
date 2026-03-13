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

## Deferred decisions & open questions

Each module maintains a `docs/{module}/DEFERRED.md` file for items that were identified during design or implementation but deferred to a follow-up. These capture context, rationale, dependencies, and proposed designs so nothing is lost.

Current deferred files:
- [docs/widget/DEFERRED.md](widget/DEFERRED.md) — Widget module (Milestone 4)

---

## Completed milestones

### M0: Project skeleton & CI ✅

Go module, directory structure, CI pipeline, dependency lock, test harness scaffolding.
3 tickets, all complete.

### M1: The DOM ✅

Node data structure, tree operations, patch engine, replace, query, snapshot/restore, event queue. Pure data, no rendering. TDD mandatory for every ticket.
7 tickets, all complete. 95%+ coverage.

### M2: MCP server ✅

Server skeleton, 6 tools (patch, replace, await_event, snapshot, restore, query), error handling. Protocol layer atop DOM, no rendering.
7 tickets, all complete.

### M3: Script runtime ✅

goja sandbox, $ API (current node + cross-node), emit(), state, lifecycle hooks, computed props. TDD mandatory for $ API surface.
7 tickets, all complete.

### M4: Widget library (v1 core set) ✅

Registry, rendering pipeline, 11 v1 widgets (container, text, input, textarea, select, button, table, list, diff, log, code). 154 tests, all passing.
12 tickets, all complete. Design doc: [docs/widget/DESIGN.md](widget/DESIGN.md). Deferred: [docs/widget/DEFERRED.md](widget/DEFERRED.md).

### M5: BubbleTea integration ✅

BubbleTea model & update loop, focus management (FocusRing with Tab/Shift-Tab, container trapping), MCP↔BubbleTea bridge (DOMChangedMsg, CallTool), terminal resize, startup & shutdown (stdio MCP + stderr BubbleTea alt screen, signal handling, broken pipe detection). 43 tests, 88% coverage on render package.
5 tickets, all complete.

---

## Active milestones

---

### Milestone 6: End-to-end demo — log viewer

**Goal**: The first reference demo runs end-to-end. Validates the full stack: MCP → DOM → scripts → widgets → terminal. A user opens a separate Claude Code session, prompts "build me a log viewer," and Claude Code constructs the TUI using imagine-tui's MCP tools.

**Design doc**: [docs/demos/log-viewer.md](demos/log-viewer.md)

#### M6-1: Demo project scaffold
- Create `demo/log-viewer/` directory
- `.mcp.json` with streamable HTTP URL config
- Generate `sample-data/app.log` — realistic log data (~500 lines, mixed severities)
- `README.md` with two-terminal setup instructions

#### M6-2: System prompt (`demo/log-viewer/CLAUDE.md`)
- Instructs Claude Code to build a log viewer using imagine-tui tools
- Example `replace` payload for initial UI tree
- Describes the `await_event` → `patch` interaction loop
- Example goja scripts for local filtering

#### M6-3: Smoke test — initial render
- Start imagine-tui server, send `replace` with the log viewer tree via HTTP, call `query` to verify DOM state
- Verifies: all node IDs exist, log widget populated, status bar computed prop works

#### M6-4: Smoke test — interaction loop
- Simulate severity button click, verify filtered log entries
- Simulate search input change, verify text filter applied
- Verify status bar count updates

#### M6-5: Manual end-to-end test
- Full two-terminal demo flow
- Iterate on system prompt until Claude Code reliably builds the UI

#### M6-6: Demo recording & polish
- Record with asciinema or screen recording
- README update with recording link
- Performance check: measure latency of replace + await_event cycle

---

### Milestone 7: Extended demos

**Goal**: Ship 2-3 more reference demos, exercising increasingly complex widget combinations and multi-tool orchestration.

#### M7-1: Test whisperer demo
Design doc: [docs/demos/test-whisperer.md](demos/test-whisperer.md)
- Multi-tool orchestration (filesystem + shell), live row status updates
- Exercises: table, code, log, button, text, container

#### M7-2: Living dashboard demo
Design doc: [docs/demos/living-dashboard.md](demos/living-dashboard.md)
- Multi-MCP-server orchestration, sparkline (may need v2 widget), container layout
- Requires sparkline widget implementation (promote from v2 if not already done)

#### M7-3: Log detective demo
Design doc: [docs/demos/log-detective.md](demos/log-detective.md)
- Most complex multi-tool orchestration: log parsing + source file reading + causal tracing
- Exercises: log, sparkline, text with annotations

---

## Backlog (unscheduled)

Items not tied to a milestone. Will be scheduled as needed.

### Container viewport & overflow scrolling
- Add `height` / `max_height` props to containers (fixed int or percentage)
- Add `overflow` prop: `"scroll"` wraps children in a bubbletea viewport
- Without this, tall content pushes the entire screen down instead of scrolling within its panel
- Discovered during live demo: log/narrative panels overflow their containers
- **Implementation**: wrap container rendering in `viewport.Model` when `overflow: "scroll"` and height is constrained
- **TDD**: golden file tests for constrained containers, overflow clipping

### Flex layout & layout managers
- Flex grow/shrink (one panel fills remaining space after siblings)
- Min/max width constraints on containers
- Wrapping grids (flow children into rows when they overflow)
- Current `width: "50%"` and `direction` cover basics but can't express elastic layouts
- **Implementation**: lipgloss `Place` + custom flex algorithm, or adopt a layout library
- **TDD**: layout calculation unit tests for grow/shrink/wrap scenarios

### Script runtime: state in computed props & cross-node state
- Make `state` readable from computed props (currently only `$` props are accessible)
- Add `$('other-node').state` for cross-node state sharing
- Eliminates the hidden-input workaround for shared reactive state
- Key enabler for client-side-heavy UIs (games, dashboards with complex local logic)
- **TDD**: computed prop reads state, cross-node state access, reactivity triggers

### Script runtime: timers (setTimeout/setInterval)
- Sandboxed short-duration timers for animations and timed events
- Combat damage flash, countdown timers, progress animations
- Must integrate with bubbletea's `tea.Tick` command pattern
- Safety: max duration cap, max concurrent timers, auto-cancel on node removal
- **TDD**: timer fires, timer cancels on remove, max limits enforced

### BubbleTea ecosystem widget integration
- Progress bar widget (from `bubbles/progress`)
- Spinner widget (from `bubbles/spinner`)
- Markdown/glamour widget (from `glamour`)
- Sparkline widget (needed for M7 living dashboard)
- Each maps to a new widget type in the registry
- **TDD**: golden file tests per new widget type

### TSX fragment runtime
- Implement the JSX transform in the goja layer
- Component registry: `<ListItem>`, `<Text>`, `<Container>` → DOM node specs
- Compile step: string transform before passing to goja, or runtime tagged template processing
- **TDD**: transform input/output pairs, component registry lookup, fragment rendering

### Session persistence
- Serialize DOM + snapshot store + script state to JSON on disk
- Resume from serialized state on startup
- **TDD**: serialize/deserialize round-trip, resume renders correctly

### Error recovery & resilience
- Script timeout handling (goja interrupt)
- Malformed patch recovery (error message shown in TUI, not crash)
- MCP reconnection after disconnect
- DOM consistency checks (orphaned nodes, dangling references)
- **TDD**: every error path, every recovery mechanism

### Dedicated agent-launched Ghostty sessions
- Launch imagine-tui in a dedicated Ghostty window/tab for a single agent session
- Treat the Ghostty process/window as the ownership and lifetime boundary
- Closing the window terminates the session and prevents further control
- Design socket ownership, launcher lifecycle, cleanup semantics, and any required session registry
- Defer until shell-run single-owner reconnect and live scripting are stable

### Documentation & packaging
- README with architecture overview, quickstart, demo walkthroughs
- MCP tool schema documentation (auto-generated from Go types)
- Go binary releases for macOS (arm64, amd64), Linux (amd64)
- Homebrew formula or `go install` instructions

---

## Milestone ordering & dependencies

```
M0 (skeleton) ✅
 └→ M1 (DOM) ✅
     ├→ M2 (MCP) ✅
     └→ M3 (scripts) ✅
         └→ M4 (widgets) ✅
              └→ M5 (BubbleTea integration) ✅
                   └→ M6 (log viewer demo)
                        └→ M7 (extended demos)
```

M1 is the critical path. M2 and M3 can proceed in parallel once M1 is complete. M4 depends on M3 (widgets need to trigger scripts). M5 depends on M2 + M4 (wires MCP and widgets into BubbleTea). M6 is the integration proof. M7 is iterative.

## Effort estimates

| Milestone | Tickets | Estimated effort | Risk |
|-----------|---------|-----------------|------|
| M0: Skeleton ✅ | 3 | 1–2 days | Low |
| M1: DOM ✅ | 7 | 2–3 weeks | Medium |
| M2: MCP ✅ | 7 | 1–2 weeks | Low |
| M3: Scripts ✅ | 7 | 2–3 weeks | High |
| M4: Widgets ✅ | 12 | 3–4 weeks | Medium |
| M5: Integration ✅ | 5 | 2–3 weeks | High (concurrency, BubbleTea bridge) |
| M6: Log viewer | 6 | 1–2 weeks | Medium (first e2e, expect surprises) |
| M7: Extended demos | 3 | 2–3 weeks | Medium |
