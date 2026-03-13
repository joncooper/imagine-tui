# Test Whisperer Demo

## Goal

Run tests, show failures with Claude-diagnosed summaries, click to see source + fix, click "Fix it" to write the patch and re-run. This is the most complex single-session demo, exercising multi-tool orchestration and live status updates.

## Widgets exercised

- **table** — failure list with sortable columns, expandable row detail, row-level status styling (red → green on fix)
- **code** — source context with line-range highlighting
- **log** — test output display
- **text** — header, status bar with computed summary
- **button** — "Fix it" action trigger
- **container** — overall layout

## External tools required

- **Filesystem MCP** — read test files and source files
- **Shell MCP** — run tests, apply patches

## MCP tool sequence

1. `replace` — initial screen: header (text), failure table (table with `row_style`), status bar (text with computed summary)
2. `await_event` — wait for user to select a row
3. User selects a row, presses Enter → event routed to Claude
4. Claude reads test file + source file (via filesystem MCP server)
5. `patch` — insert detail panel: code widget (showing source), text annotation, button ("Fix it")
6. `await_event` — wait for "Fix it" click
7. User clicks "Fix it" → event to Claude
8. Claude writes file (via filesystem), runs test (via shell)
9. `patch` — update row status from red to green
10. Back to step 2

## Local scripts (goja)

- **Table sort/filter**: sort by file name, test name, or severity
- **Computed summary**: `"3 of 14 fixed"` — updates automatically as rows change status via computed prop
- **Status bar elapsed time**: computed from `on_tick` hook

## Mock test runner

For deterministic testing, a mock test runner produces canned failures:
- Fixed set of ~14 test failures across 3-4 files
- Each failure has: test name, file path, line number, error message, expected vs actual
- Running a "fixed" test returns success

## Tickets (preserved from original M6 design)

### Demo script & system prompt
- Write the Claude Code system prompt / CLAUDE.md instructions for the test whisperer demo
- Define the MCP tool call sequence: initial `replace` with table + status bar, `await_event` loop
- Write a mock test runner that produces canned failures for deterministic testing
- Document the expected interaction flow

### Initial screen build
- Claude Code calls `replace` with: header (text), failure table (table with row_style), status bar (text with computed summary)
- Verify the full render pipeline: MCP → DOM → widgets → terminal
- **Smoke test**: canned replace payload renders correctly

### Row interaction
- User selects a row, presses Enter → event routed to Claude
- Claude reads test file + source file (via filesystem MCP server)
- Claude patches in a detail panel (code widget + text annotation + button)
- **Smoke test**: canned event → canned patch → correct render

### Fix-it flow
- User clicks "Fix it" → event to Claude
- Claude writes file (via filesystem), runs test (via shell), patches row status
- Row transitions from red to green with animation (style change)
- **Smoke test**: full cycle with mock filesystem and shell

### Local scripts
- Table sort/filter scripts (goja)
- Computed summary: "3 of 14 fixed" updates automatically as rows change status
- Status bar shows elapsed time (computed from `on_tick`)
- **Tests**: sort script, filter script, computed summary accuracy

### Demo polish & recording
- Record a terminal session (asciinema or similar) for the README
- Write setup instructions: how to configure Claude Code MCP, run the server, trigger the demo
- Performance profiling: measure latency at each tier

## Smoke tests

- Scripted test: start server, send canned `replace` payload, verify DOM via `query`
- Scripted test: send canned event, send canned `patch`, verify render output
- Scripted test: full fix-it cycle with mock filesystem and shell
