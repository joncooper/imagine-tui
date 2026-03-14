# Log Viewer Demo (Milestone 6)

The first end-to-end demo of imagine-tui. Validates the full stack: MCP → DOM → scripts → widgets → terminal.

## Goal

User opens a separate Claude Code session, prompts "build me a log viewer for the sample data," and Claude Code constructs a live TUI using imagine-tui's MCP tools. The log viewer displays log entries with severity filtering, text search, and Claude-powered error explanation.

## Isolation architecture

Unix domain socket transport separates the TUI terminal from the Claude Code session. The MCP Go SDK v1.4.0 provides `IOTransport{Reader, Writer}` which wraps any `io.ReadWriteCloser`, and `Server.Connect()` for per-connection sessions.

```
Terminal A                          Terminal B
┌──────────────────────┐           ┌──────────────────────┐
│ imagine-tui serve    │           │ claude               │
│   --socket *.sock    │◄──Unix──► │   (demo session)     │
│                      │  socket   │                      │
│ BubbleTea renders    │           │ Reads CLAUDE.md,     │
│ the TUI on stdout    │           │ calls MCP tools,     │
│                      │           │ builds the log viewer│
└──────────────────────┘           └──────────────────────┘
```

- **Terminal A**: `imagine-tui serve --socket /tmp/imagine-tui.sock` — owns the terminal for BubbleTea rendering, listens on a Unix socket for MCP
- **Terminal B**: `cd demo/log-viewer && claude` — separate Claude Code session with project-scoped `.mcp.json`
- **Zero interference** with development sessions — Claude Code spawns `imagine-tui connect <path>` as a stdio subprocess that bridges to the socket

### Why Unix domain sockets?

- No port allocation or HTTP overhead — same-host IPC only
- Clean separation: BubbleTea owns Terminal A's stdout, MCP uses a Unix socket
- No `/dev/tty` conflicts (two TUI apps can't share a terminal)
- No tmux/screen dependency
- Claude Code supports stdio MCP servers via `"command"`/`"args"` in `.mcp.json`; the `connect` subcommand bridges stdio ↔ socket transparently

## Widgets exercised

- **container** — nested flex layout (vertical root + horizontal main)
- **text** — header, severity filter label, status bar with computed props
- **log** — main log display with severity coloring, timestamps, sticky-bottom auto-scroll
- **button** — severity filter toggles (DEBUG, INFO, WARN, ERROR)
- **input** — text search/filter

## UI structure

Built by Claude Code via a single `replace` call:

```
container (id: "root", direction: vertical)
├── text (id: "header", content: "Log Viewer — app.log")
├── container (id: "main", direction: horizontal)
│   ├── container (id: "sidebar", width: 25%)
│   │   ├── text (id: "filter-label", content: "Severity Filter")
│   │   ├── button (id: "sev-debug", label: "DEBUG")
│   │   ├── button (id: "sev-info",  label: "INFO")
│   │   ├── button (id: "sev-warn",  label: "WARN")
│   │   ├── button (id: "sev-error", label: "ERROR")
│   │   └── input (id: "search", placeholder: "Search...")
│   └── log (id: "log-view", width: fill, sticky_bottom: true)
└── text (id: "status-bar", computed: { "content": "state.filtered + ' of ' + state.total + ' entries'" })
```

## Interaction flow

### Initial setup
1. Claude reads `sample-data/app.log` via built-in filesystem access
2. Claude calls `replace` with the full UI tree (above)
3. Log widget is populated with parsed log entries
4. Claude calls `await_event` — blocks until user interaction

### Local interactions (instant, via goja scripts)
5. **Severity button click** → `on_event` script toggles that severity level on/off, filters log entries, patches log widget via `emit('local', patchOps)`. Status bar updates via computed prop.
6. **Search input change** → `on_change` script filters entries matching the search term, updates log widget locally.

### Claude-routed interactions (1-3s round trip)
7. **Select a log entry** (if supported) → `emit('agent', { line, severity, message })` → Claude explains the error, reads source files for context
8. Claude calls `patch` to insert an explanation panel below the log
9. Back to `await_event`

## Local scripts (goja)

### Severity filter (`on_event` on each severity button)
```javascript
// Toggles severity, filters entries, updates log widget
var sev = $.props.label;
state.filters = state.filters || { DEBUG: true, INFO: true, WARN: true, ERROR: true };
state.filters[sev] = !state.filters[sev];

var allEntries = state.allEntries || [];
var filtered = allEntries.filter(function(e) { return state.filters[e.severity]; });
state.filtered = filtered.length;

emit('local', [{ op: 'update', id: 'log-view', props: { lines: filtered } }]);
```

### Text search (`on_change` on search input)
```javascript
var term = $.value.toLowerCase();
var allEntries = state.allEntries || [];
var filtered = allEntries.filter(function(e) {
  return e.message.toLowerCase().indexOf(term) !== -1;
});
state.filtered = filtered.length;

emit('local', [{ op: 'update', id: 'log-view', props: { lines: filtered } }]);
```

### Computed status bar
```javascript
state.filtered + ' of ' + state.total + ' entries'
```

## External tools required

- **Filesystem** — built into Claude Code, used to read `sample-data/app.log`
- No shell, git, or other MCP servers needed — this is the simplest possible demo

## Demo project structure

```
demo/log-viewer/
  CLAUDE.md             # System prompt for the demo Claude Code session
  .mcp.json             # MCP server config: { "mcpServers": { "imagine-tui": { "command": "imagine-tui", "args": ["connect", "/tmp/imagine-tui.sock"] } } }
  sample-data/
    app.log             # ~500 lines, mixed severities, realistic timestamps and messages
  README.md             # How to run the demo (two-terminal setup)
```

## Sample data format

`sample-data/app.log` should contain ~500 lines of realistic application logs:

```
2026-03-13T10:00:01.234Z INFO  [server] Starting HTTP server on :8080
2026-03-13T10:00:01.456Z DEBUG [db] Connection pool initialized (max=10)
2026-03-13T10:00:02.789Z INFO  [auth] OAuth2 provider configured
2026-03-13T10:00:05.123Z WARN  [cache] Redis connection timeout, retrying...
2026-03-13T10:00:05.456Z ERROR [cache] Redis connection failed after 3 retries
2026-03-13T10:00:06.789Z INFO  [server] Health check endpoint ready
...
```

Requirements:
- Timestamps in ISO 8601 format
- Severity levels: DEBUG, INFO, WARN, ERROR (roughly 40% INFO, 30% DEBUG, 20% WARN, 10% ERROR)
- Component tags in brackets: `[server]`, `[db]`, `[auth]`, `[cache]`, `[api]`, `[worker]`
- Realistic messages including stack traces for ERROR entries
- Some multi-line entries (stack traces, JSON payloads)

## Smoke tests

### Initial render test
Start imagine-tui server, connect via Unix socket, send `replace` with the log viewer tree, call `query`:
- All node IDs exist in the DOM
- Log widget has entries
- Status bar computed prop evaluates correctly

### Interaction test
- Simulate severity button click → verify log entries filtered
- Simulate search input change → verify text filter applied
- Verify status bar count updates

## How to run

1. Build imagine-tui: `go build -o imagine-tui ./cmd/imagine-tui` and ensure it's on `$PATH`
2. Start the server: `./imagine-tui serve --socket /tmp/imagine-tui.sock` (Terminal A)
3. Open a new terminal, navigate to the demo: `cd demo/log-viewer` (Terminal B)
4. Start Claude Code: `claude` (Terminal B)
5. Prompt: "Build me a log viewer for the sample data in sample-data/app.log"
6. Watch the TUI appear in Terminal A
