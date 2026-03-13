# Imagine TUI: Claude Code-driven terminal user interfaces via MCP

## What this is

An MCP server that renders interactive terminal UIs. Claude Code connects to it the same way it connects to any other MCP server — filesystem, git, databases — and drives it through tool calls. Claude Code builds the UI, writes local scripts for snappy interactions, and handles complex decisions on each round-trip. The user experiences a reactive terminal application. The TUI server has no AI in it. It's a rendering engine, a script runtime, and an event queue, exposed over MCP.

This is the TUI equivalent of the "Imagine with Claude" web Visualizer: Claude generates interactive UI, a runtime renders it, user actions feed back to Claude when needed. The difference is that the runtime is an MCP server, and the "Claude" driving it is Claude Code — which means the TUI becomes a frontend for Claude Code's entire tool repertoire.

## Architecture

```
┌─────────────────────────────────────────┐
│            Claude Code                   │
│  (MCP client, reasoning, orchestration)  │
│                                          │
│  Calls tools on ANY connected server:    │
│  - imagine_tui.patch(...)                │
│  - imagine_tui.await_event()             │
│  - filesystem.read_file(...)             │
│  - shell.exec(...)                       │
│  - database.query(...)                   │
│  - any other MCP server                  │
└──────────────┬───────────────────────────┘
               │ MCP (stdio or SSE)
               │
┌──────────────▼───────────────────────────┐
│         Imagine TUI server               │
│                                          │
│  ┌─────────────────────────────────────┐ │
│  │          TUI DOM tree               │ │
│  │  Persistent node tree with IDs,     │ │
│  │  props, scripts, computed props     │ │
│  └─────────────────────────────────────┘ │
│  ┌───────────────┐ ┌────────────────┐    │
│  │ goja scripts  │ │ Snapshot store │    │
│  │ (reactive     │ │ (named DOM     │    │
│  │  tier, <1ms)  │ │  checkpoints)  │    │
│  └───────────────┘ └────────────────┘    │
│  ┌─────────────────────────────────────┐ │
│  │ BubbleTea render loop (16ms)       │ │
│  │ Lip Gloss styling, terminal I/O    │ │
│  └─────────────────────────────────────┘ │
│  ┌─────────────────────────────────────┐ │
│  │ Event queue                         │ │
│  │ Claude-routed events wait here     │ │
│  │ until await_event is called        │ │
│  └─────────────────────────────────────┘ │
└──────────────┬───────────────────────────┘
               │ ANSI to stdout
               │
┌──────────────▼───────────────────────────┐
│            Terminal                       │
│  User sees and interacts with the TUI    │
└──────────────────────────────────────────┘
```

### What lives where

**Claude Code** (the client): All reasoning, all decisions, all data fetching. Maintains conversation context. Orchestrates multi-tool workflows. Writes scripts for the TUI. Has no rendering logic.

**Imagine TUI server**: Pure rendering engine + script runtime + event queue. Has no API key, no HTTP client, no AI. Knows how to: maintain a DOM tree, run goja scripts, render via BubbleTea/Lip Gloss, queue events for Claude, and expose all of this over MCP.

**The key insight**: Between an `await_event` return and the next `patch` call, Claude Code can do anything — read files, run shell commands, query databases, call other MCP servers. The TUI is just another tool in Claude Code's belt. This means the TUI is a frontend for the entire development environment.

## The TUI DOM

The server maintains a persistent tree of UI nodes in memory. Each node has:

- **id**: Stable string identifier. Claude references these across turns. (e.g., `"header"`, `"search_input"`, `"results_table"`)
- **type**: One of the registered widget types (container, text, input, select, button, table, list, diff, log, tabs, progress, sparkline, tree, etc.)
- **props**: Type-specific properties (content, placeholder, options, rows, style, etc.)
- **children**: Ordered child node references (for container types)
- **scripts**: Claude-authored reactive expressions attached to event hooks
- **computed**: Reactive derived props that auto-recompute when dependencies change
- **state**: Local mutable state owned by the runtime (cursor position, scroll offset, focus, selection) — Claude doesn't manage this directly

Nodes are rendered by mapping each type to a Bubbles component + Lip Gloss styling. The DOM is the abstraction boundary: Claude thinks in semantic nodes, the server thinks in terminal output.

## MCP tools

The server exposes these tools over MCP:

### patch
The workhorse. An ordered list of atomic operations applied in a single pass:
- `update` — change props/scripts on an existing node by ID
- `insert` — add a new node as a child of a parent, optionally positioned relative to a sibling
- `remove` — delete a node by ID
- `move` — reparent or reorder a node

Returns: `{ ok: true }` on success, error details on failure.

Most Claude Code responses are 1–5 patch ops. The server applies them atomically and re-renders only affected subtrees. Claude Code streams its response, and the MCP framework sends each patch tool call as it completes — the user sees the UI build up progressively.

### replace
Replace an entire subtree. Used for initial screen construction or full view transitions where the structural change is too large for incremental patches.

Returns: `{ ok: true }` on success.

### await_event
**The core loop mechanism.** A long-poll: the server blocks until a Claude-routed event fires, then returns the event plus contextual DOM state. Between this returning and the next tool call, Claude Code can do anything — read files, run commands, call other MCP servers — then patch the TUI with results.

Supports optional parameters:
- `timeout_ms` — return `{ timeout: true }` after N milliseconds if no event fires. Enables polling patterns and background refresh loops.
- `filter` — array of node IDs to listen to. Events from other nodes are held in the queue for the next unfiltered call. Enables focused interaction flows.
- `debounce_ms` — coalesce rapid events (e.g., multiple filter toggles) into a single return containing the last event plus a `coalesced_count` field. Prevents flooding Claude with redundant round-trips.

Returns:
```json
{
  "event": "click",
  "source": "submit_btn",
  "context": {
    "name_input": { "value": "Acme Corp" },
    "amount_input": { "value": "15000" }
  },
  "dom_summary": "root > header + form(name_input, amount_input, submit_btn) + results_table"
}
```

The `context` field auto-includes sibling and parent node state relevant to the event source. The `dom_summary` is a compact structural representation, not a full serialization — keeps Claude Code's context window lean.

### snapshot
Name the current DOM state. The server deep-copies the full tree + local state (scroll positions, input values, focus) into a named checkpoint. Returns: `{ ok: true, name: "..." }`.

### restore
Rewind to a named snapshot. The server swaps the tree back and re-renders. Enables undo, branching, and "try something risky then roll back." Returns: `{ ok: true, restored: "..." }`.

### query
Read back current state of specific nodes, including local state Claude doesn't normally see (what the user has typed, scroll positions, selection state). Used before complex patches when the auto-included event context isn't sufficient.

Returns: node state keyed by ID.

## The event loop

```
1. Claude Code calls  imagine_tui.replace(initial_tree)
2. Claude Code calls  imagine_tui.await_event()          ← blocks
3.   ...user interacts locally (typing, scrolling)...
4.   ...goja scripts run (validation, formatting)...
5.   ...user triggers a Claude-routed action...
6.   ...server returns event + context...
7. Claude Code reasons about the event
8. Claude Code optionally calls OTHER tools
   (filesystem.read, shell.exec, db.query, etc.)
9. Claude Code calls  imagine_tui.patch(updates)
10. goto 2
```

This is the Elm architecture, but Update is Claude Code and the message bus is MCP.

## Three-tier execution model

### Tier 1: BubbleTea frame loop (~16ms)
Handles: cursor movement, text input, scroll, focus, key repeat, Lip Gloss styling, ANSI rendering, terminal I/O. Standard BubbleTea Update/View cycle. Never waits on anything async.

### Tier 2: goja script runtime (<1ms)
Handles: validation, computed props, conditional show/hide, local filtering, formatting, state machines, local DOM patches. Scripts are authored by Claude Code but execute locally with zero latency.

### Tier 3: Claude Code (1–3s, via MCP)
Handles: new screen construction, complex decisions, ambiguous user intent, data-dependent logic, multi-tool workflows (read a file then update the UI), recovery from errors.

### Event routing
Each node's event hooks declare their tier:
- `"local"` — handled entirely by BubbleTea (focus, scroll, cursor)
- Script function body — runs in goja instantly
- `"agent"` — queued for the next `await_event` call, which routes to Claude Code

### Event coalescing
Rapid user actions (toggling multiple filter checkboxes, repeated keypresses) can generate a burst of Claude-routed events. The `debounce_ms` parameter on `await_event` coalesces these into a single return, preventing wasteful round-trips. The server holds events for the debounce window and returns the most recent, with a `coalesced_count` field so Claude knows it missed intermediate states and can query if needed.

## Scripting model

### Script attachment
Scripts live on nodes, set by Claude Code via patch ops. They attach to lifecycle/event hooks:
- `on_mount` — runs when node enters the DOM
- `on_change` — runs when the node's value changes
- `on_event` — runs on any event from child nodes (bubbling)
- `on_focus` / `on_blur` — focus lifecycle
- `on_key` — specific keypress handling

### The $ API
Scripts get a minimal sandboxed API:

```
$              — current node
$.value        — read/write value
$.props        — read/write props
$.style        — shorthand for style prop
$.children     — child node list

$('id')        — reach any node by ID
$('id').text   — read/write text content
$('id').visible — show/hide
$('id').rows   — table-specific accessors

emit('local', patch)   — apply DOM patch instantly
emit('agent', data)   — queue event for Claude Code

state                  — persistent script-local store
state.x ??= 0         — survives across script invocations
```

### Computed props
Reactive derivations declared on nodes. The runtime tracks dependencies and re-evaluates when upstream values change:

```json
{
  "id": "total",
  "type": "text",
  "computed": {
    "text": "return '$' + (Number($('qty').value) * Number($('price').value)).toLocaleString()"
  }
}
```

### TSX-ish fragments
For richer local rendering, scripts can emit JSX-like component fragments that the runtime instantiates as DOM nodes:

```js
$('results').render(
  items.filter(i => i.name.includes(query)).map(i =>
    <ListItem key={i.id} label={i.name} on_click={`state.sel='${i.id}'`} />
  )
)
```

### Sandboxing
goja is configured with no I/O primitives: no `require`, no `fetch`, no filesystem, no timers (the runtime provides `on_tick` as a hook instead). Scripts can read/write the DOM, keep local state, and `emit`. The only door to the outside world is `emit('agent', ...)`.

## Widget library

### Core types (v1)

These are required for the launch demos. All seven reference demos depend on this set.

- **container** — flex-like layout (horizontal/vertical), borders, padding. The structural backbone.
- **text** — styled text content, Lip Gloss format tokens. Supports inline markup for mixed styles (e.g., annotations with severity-colored tags).
- **input** — single-line text input (Bubbles textarea).
- **textarea** — multi-line text input.
- **select** — single/multi-select from options list.
- **button** — focusable, styled, triggers events.
- **table** — rows + columns, sortable, scrollable. Row-level status styling (e.g., pass/fail coloring). Expandable row detail.
- **list** — vertical item list with selection, badges, and status indicators.
- **diff** — split or unified diff view with hunk-aware navigation, line numbers, add/remove coloring. Required by: migration pilot, PR review cockpit.
- **log** — append-only scrolling text with ANSI passthrough, auto-scroll, severity-level coloring. Required by: test whisperer (test output), log detective, living dashboard (build logs).
- **code** — syntax-highlighted code block with line numbers and line-range highlighting. Required by: test whisperer (source context), PR review cockpit, migration pilot.

### Extended types (v2)
- **tabs** — tab bar + content panels
- **progress** — progress bar / spinner
- **sparkline** — inline data visualization (may promote to v1 if dashboard demo is prioritized)
- **tree** — collapsible tree view with status badges per node. Required by: migration pilot (file tree).
- **modal** — overlay container
- **form** — logical grouping with aggregate validation state

### Default behaviors
Widgets ship with sensible defaults that work without scripting:
- Inputs auto-validate against `pattern` prop (regex)
- Tables handle column sort locally when `sortable: true`
- Tables support row expansion with `expandable: true` (goja script provides detail content)
- Lists support arrow-key navigation and type-to-filter
- Containers handle focus cycling among focusable children
- Selects filter options by typed prefix
- Tabs switch panels on arrow keys
- Modals trap focus and dismiss on Escape
- Log auto-scrolls to bottom unless user has scrolled up (sticky-bottom)
- Diff supports `d` to toggle split/unified, `n`/`p` for next/prev hunk
- Code supports `y` to yank line range to clipboard

Claude Code can override any default by attaching a script.

## Reference demos

These seven demos validate the architecture and define the v1 widget requirements:

1. **Test whisperer** — Run tests, show failures with Claude-diagnosed summaries, click to see source + fix, click "Fix it" to write the patch and re-run. Exercises: table, code, log, multi-tool (shell + filesystem), live row status updates.

2. **Living dashboard** — Claude reads package.json, git log, GitHub issues, CI status, TODO comments. Builds multi-panel dashboard with sparklines, tables, gauges. Click any metric for Claude's explanation. Exercises: container layout, sparkline, table, text, multi-MCP-server orchestration (filesystem + git + GitHub).

3. **Database explorer** — Natural language → SQL → results table. Sort/filter locally. Drill into any row for Claude-narrated detail. Exercises: input, table with expandable rows, computed props, schema-aware query generation.

4. **Migration pilot** — File tree with status badges + split diff view. Accept/reject/modify per file. Claude accumulates style preferences across files. Exercises: tree, diff, button, snapshot/restore, context accumulation.

5. **Log detective** — Pipe logs in, Claude structures them: timeline, severity heatmap, anomaly panel with causal tracing. Click a spike to scroll to that moment. Exercises: log, sparkline, text with inline annotations, cross-file source reasoning.

6. **API workbench** — Describe requests in English, Claude reads OpenAPI spec, constructs request, shows for review, fires on confirm. Chains responses (user ID from create → order endpoint). Exercises: code (JSON display), input, button, local JSON fold/expand scripts.

7. **PR review cockpit** — Read diff + issues + changed files. Per-hunk annotations tagged by category. Filter by category (local script). Draft review comments in user's voice. Exercises: diff, list with filtering, code, text with annotations, event coalescing (rapid filter toggles).

## Implementation stack

- **Go**: main runtime, BubbleTea app, MCP server
- **goja**: embedded JS engine for script tier (github.com/dop251/goja)
- **Lip Gloss**: terminal styling
- **Bubbles**: reusable TUI components
- **MCP Go SDK**: server-side MCP implementation (stdio transport for Claude Code)

The server ships as a single Go binary. Claude Code's MCP config points at it:

```json
{
  "mcpServers": {
    "imagine-tui": {
      "command": "imagine-tui",
      "args": ["serve"]
    }
  }
}
```

## Open questions

1. **MCP streaming granularity**: Can Claude Code stream individual patch ops within a single tool call, or does the full tool call need to complete before the server sees it? This affects progressive rendering.
2. **Notifications / server-initiated messages**: MCP supports server→client notifications. Could the TUI server push updates to Claude Code (e.g., "a background script finished") without waiting for `await_event`?
3. **JSX transform**: How much of JSX do we support in goja? Minimal (components → function calls) or more complete?
4. **Style system**: Lip Gloss DSL vs. higher-level token system (`"bold danger"`) that maps to Lip Gloss?
5. **Layout model**: Pure Lip Gloss flexbox-like, or something closer to CSS grid? Terminal resize handling?
6. **Session persistence**: Can the DOM + snapshot store be serialized to disk and resumed?
7. **Multi-pane**: Could multiple TUI server instances run in tmux panes, each driven by the same Claude Code session?
8. **Escape hatch**: Keyboard shortcut to drop into raw Claude Code chat mode, then return to the TUI?