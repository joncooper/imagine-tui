# CLAUDE.md

## Project
Imagine TUI — an MCP server that renders interactive terminal UIs driven by Claude Code.
Full spec: docs/SPEC.md
Backlog: docs/BACKLOG.md

## Development rules
- Go 1.22+, module name: github.com/joncooper/imagine-tui
- TDD is mandatory for internal/dom/ and internal/script/ — write the test first, confirm it fails, then implement
- Golden file tests for widget rendering — use testdata/golden/, regenerate with GOLDEN_UPDATE=1
- Run `go test ./...` and `golangci-lint run` before considering any task complete
- No code without tests in dom/ or script/ packages — this is non-negotiable

## Architecture layers (read SPEC.md for full detail)
- internal/dom/ — pure data: tree, patch, snapshot, query, event queue. Zero dependencies on rendering or MCP.
- internal/script/ — goja integration, $ API, computed props. Depends on dom/.
- internal/widget/ — Lip Gloss renderers per widget type. Depends on dom/ + script/.
- internal/mcp/ — MCP server, tool handlers. Depends on dom/.
- internal/render/ — BubbleTea model, wires everything together. Depends on all.
- cmd/imagine-tui/ — main entry point.

## Style
- Table-driven tests preferred
- Error messages should include the node ID and operation that failed
- No panics in library code — return errors
- Contexts for cancellation on anything that blocks