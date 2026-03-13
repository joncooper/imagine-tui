# Living Dashboard Demo

## Goal

Claude reads project metadata (package.json, git log, GitHub issues, CI status, TODO comments) and builds a multi-panel dashboard with sparklines, tables, and gauges. Click any metric for Claude's explanation. Validates multi-MCP-server orchestration and complex container layout.

## Widgets exercised

- **container** — multi-panel grid layout (the core challenge)
- **table** — issues list, CI results, dependency versions
- **text** — metric labels, Claude explanations
- **sparkline** — may need v2 widget promotion; trend lines for commit frequency, issue velocity, build times

## External tools required

- **Filesystem MCP** — read package.json, scan for TODO comments
- **Git MCP** — git log, branch status, contributor stats
- **GitHub MCP** — issues, PRs, CI status

This is the most demanding demo for **multi-MCP-server orchestration**: Claude must call 3+ MCP servers, aggregate the data, and build a coherent dashboard.

## MCP tool sequence

1. Claude reads project metadata via filesystem, git, and GitHub MCP servers
2. `replace` — build multi-panel dashboard layout
3. `await_event` — wait for user to click a metric
4. User clicks a metric → event to Claude
5. Claude fetches additional detail (e.g., recent commits for a sparkline spike)
6. `patch` — insert explanation panel
7. Back to `await_event`

## Local scripts (goja)

- **Auto-refresh**: periodic data re-fetch via `on_tick` (if supported)
- **Panel collapse/expand**: local script toggles panel visibility

## Key challenges

- Sparkline widget may not exist in v1 core set — needs promotion from v2 or placeholder
- Container layout must handle 4-6 panels in a responsive grid
- Data aggregation from multiple MCP servers requires careful prompt engineering

## Smoke tests

- Scripted test: send canned dashboard `replace` payload, verify all panels render
- Scripted test: click a metric, verify explanation panel appears
