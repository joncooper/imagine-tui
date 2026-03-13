# Log Detective Demo

## Goal

Pipe logs in, Claude structures them: timeline, severity heatmap, anomaly panel with causal tracing. Click a spike to scroll to that moment. This is the most complex multi-tool orchestration demo, combining log parsing, source file reading, and causal reasoning.

## Widgets exercised

- **log** — main log display with severity coloring, timestamps, ANSI passthrough
- **text** — annotations with inline highlights, causal trace explanations
- **sparkline** — severity heatmap / timeline visualization (may need v2 widget)
- **container** — split layout: timeline + log + detail panel

## External tools required

- **Filesystem MCP** — read source files for causal tracing
- **Shell MCP** — potentially pipe live logs

## MCP tool sequence

1. Claude reads log file(s) via filesystem
2. `replace` — build layout: timeline (sparkline), log view, anomaly panel
3. Claude analyzes log patterns, identifies anomalies
4. `patch` — annotate anomalies in the log, populate timeline
5. `await_event` — wait for user interaction
6. User clicks a spike on the timeline → event to Claude
7. Claude scrolls log to that moment, reads source files for context
8. `patch` — insert causal trace panel with source context
9. Back to `await_event`

## Local scripts (goja)

- **Severity filtering**: toggle severity levels locally
- **Timeline scrubbing**: scroll log to match timeline position
- **Anomaly highlighting**: highlight log entries that match anomaly patterns

## Key challenges

- Cross-file source reasoning: Claude must connect log entries to source code locations
- Timeline visualization: may need sparkline or a custom timeline widget
- Log volume: must handle 1000+ lines efficiently (log widget's `max_lines` and `sticky_bottom`)
- Most complex multi-tool orchestration of all demos

## Smoke tests

- Scripted test: send canned log data via `replace`, verify log widget renders
- Scripted test: click timeline spike, verify log scrolls and detail panel appears
