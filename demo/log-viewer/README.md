# Log Viewer Demo

First end-to-end demo of imagine-tui. A user prompts Claude Code to build a log viewer, and Claude Code constructs the TUI using imagine-tui's MCP tools.

## Prerequisites

- imagine-tui binary built: `go build -o imagine-tui ./cmd/imagine-tui` (from repo root)
- Binary on `$PATH` (or use an absolute path in `.mcp.json`)
- Claude Code CLI installed

## How to run

This demo uses two terminals. The imagine-tui server renders the TUI in one terminal and accepts MCP tool calls over a Unix socket from Claude Code running in the other.

### Terminal A — Start the TUI server

```bash
cd <repo-root>
./imagine-tui serve --socket /tmp/imagine-tui.sock
```

The server will start and show a blank TUI, listening for MCP connections on the Unix socket.

### Terminal B — Start Claude Code

```bash
cd demo/log-viewer
claude
```

Claude Code will read `CLAUDE.md`, discover the imagine-tui MCP server via `.mcp.json`, and wait for your prompt. The `.mcp.json` tells Claude Code to spawn `imagine-tui connect /tmp/imagine-tui.sock` as a stdio subprocess that bridges to the Unix socket.

### Prompt Claude Code

```
Build me a log viewer for the sample data in sample-data/app.log
```

Claude Code will:
1. Read the sample log file
2. Call `imagine_tui.replace` to construct the UI (log widget, severity filters, search input, status bar)
3. Call `imagine_tui.await_event` to wait for your interaction
4. Respond to button clicks and search input with filtered views

Watch Terminal A — the TUI will appear and update as Claude builds it.

## Architecture

```
Terminal A                          Terminal B
┌──────────────────────┐           ┌──────────────────────┐
│ imagine-tui serve    │           │ claude               │
│   --socket *.sock    │◄──Unix──► │   (this session)     │
│                      │  socket   │                      │
│ BubbleTea renders    │           │ Reads CLAUDE.md,     │
│ the TUI on stdout    │           │ calls MCP tools      │
└──────────────────────┘           └──────────────────────┘
```

Terminal isolation is achieved via **Unix domain socket transport**: the TUI server owns Terminal A for rendering (stdout), while MCP communication happens over a Unix socket. Claude Code spawns `imagine-tui connect <path>` which bridges stdio to the socket, so from Claude Code's perspective it looks like a normal stdio MCP server.
