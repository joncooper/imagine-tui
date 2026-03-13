# Feedback: Spaceship Launch Demo Analysis — 2025-03-13

## Context

Claude Code was given the imagine-tui MCP tools and told to "surprise me with a demo." It autonomously built a spaceship launch dashboard with countdown timer, status panels, and scrolling event log. This analysis identifies gaps and enhancements based on what that demo pattern would exercise against the current widget system.

## Learnings

### 1. Missing widget: progress bar / gauge

A launch countdown naturally wants progress indicators — fuel levels, system readiness %, countdown progress. Right now the LLM has to fake this with text widgets and Unicode block characters like `▓▓▓▓░░░░ 40%`. A `progress` widget would be trivial to implement and very high-leverage for dashboards.

**Recommendation**: Implement a `progress` widget. Props: `value` (0-100), `label`, `style`, `show_percent` (bool). Small implementation, big impact.

### 2. No auto-refresh / timer mechanism

A countdown timer requires periodic updates. The only way to do this today is the LLM calling `patch` in a loop (burning tokens each cycle) or abusing `await_event` with `timeout_ms` as a poor man's timer. Options:

- A server-side `set_timer` tool that fires tick events the LLM can ignore
- A goja `on_tick` script hook (already mentioned in the living-dashboard design doc)

**Recommendation**: Backlog for now. The `await_event` timeout loop works, and the goja script layer (M3) already supports lifecycle hooks that could be extended.

### 3. Text alignment prop

Status panels showing `System: NOMINAL` with values right-aligned need careful manual padding. The `text` widget could support an `align` prop (`"left"`, `"center"`, `"right"`).

**Recommendation**: Small enhancement, add to text widget.

### 4. The "status table" is THE killer pattern

A table of `{system: "Engines", status: "NOMINAL", value: "100%"}` with `row_style` mapping status to colors is exactly what the table widget + `set_items` was built for. The spaceship demo validates this pattern works well. The `row_style` conditional styling feature is powerful here.

### 5. Append-only log + status panels = the canonical dashboard

The spaceship demo is essentially: header + horizontal container (status table | log widget). This pattern should be our template for dashboard-style demos going forward.

## Priority

| Enhancement | Effort | Impact | Priority |
|-------------|--------|--------|----------|
| Progress bar widget | Small (1 file + tests) | High — fills biggest gap for dashboards | **P1** |
| Text align prop | Tiny | Medium — convenience for status panels | P2 |
| Timer / auto-refresh | Medium | Medium — saves tokens but await_event works | Backlog |
| Sparkline widget | Medium | Medium — needed for living-dashboard demo (M7) | M7 |
