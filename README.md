# Imagine TUI

An MCP server that renders interactive terminal UIs driven by Claude Code.

Claude Code connects to Imagine TUI like any other MCP server and drives it through tool calls — building UIs, writing local scripts for snappy interactions, and handling complex decisions on each round-trip. The user experiences a reactive terminal application. The server has no AI in it: it's a rendering engine, a script runtime, and an event queue, exposed over MCP.

## Architecture

```
Claude Code (MCP client)
  │
  │ MCP (stdio)
  ▼
Imagine TUI server
  ├── TUI DOM tree (persistent node tree with IDs, props, scripts)
  ├── goja scripts (reactive tier, <1ms local interactions)
  ├── Snapshot store (named DOM checkpoints)
  ├── BubbleTea render loop (16ms, Lip Gloss styling)
  └── Event queue (Claude-routed events wait for await_event)
  │
  │ ANSI to stdout
  ▼
Terminal
```

### Three-tier execution model

| Tier | Latency | Handles |
|------|---------|---------|
| BubbleTea frame loop | ~16ms | Cursor, text input, scroll, focus, styling, ANSI rendering |
| goja script runtime | <1ms | Validation, computed props, show/hide, local filtering, formatting |
| Claude Code (via MCP) | 1–3s | Screen construction, complex decisions, multi-tool workflows |

## MCP Tools

- **patch** — atomic list of insert/update/remove/move operations
- **replace** — replace an entire subtree
- **await_event** — long-poll for Claude-routed user events
- **snapshot** — save named DOM checkpoint
- **restore** — rewind to a named snapshot
- **query** — read back current node state

## Project layout

```
cmd/imagine-tui/       Main entry point
internal/dom/          DOM tree, patch, snapshot, query, event queue (pure data)
internal/script/       goja integration, $ API, computed props
internal/widget/       Lip Gloss renderers per widget type
internal/mcp/          MCP server and tool handlers
internal/render/       BubbleTea model, wires everything together
testdata/golden/       Golden files for widget rendering tests
docs/                  SPEC.md, BACKLOG.md
```

## Dependencies

- [BubbleTea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) — reusable TUI components
- [goja](https://github.com/dop251/goja) — embedded JavaScript engine

## Development

```bash
make build          # Build the binary
make test           # Run all tests
make lint           # Run golangci-lint
make golden-update  # Regenerate golden files (GOLDEN_UPDATE=1)
```

Requires Go 1.22+ and [golangci-lint](https://golangci-lint.run/welcome/install/).

See [docs/SPEC.md](docs/SPEC.md) for the full specification and [docs/BACKLOG.md](docs/BACKLOG.md) for the development backlog.
