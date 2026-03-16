# OpenTelemetry

`imagine-tui` can export traces and logs to any OTLP receiver. The simplest local setup is [`otel-tui`](https://github.com/ymtdzzz/otel-tui): one small process that accepts OTLP traffic and shows traces, logs, and metrics in a terminal UI.

This is useful when you want to watch:

- the agent session itself
- the `imagine-tui connect` bridge process the agent starts as an MCP server
- the long-running `imagine-tui serve` renderer process

Codex, Claude Code, and `imagine-tui` do not emit exactly the same signal types:

- `imagine-tui` exports traces and logs
- Codex exports traces, logs, and metrics
- Claude Code exports logs/events and metrics

Because Claude Code does not currently export spans, you should expect full span trees from `imagine-tui` and Codex, plus related Claude telemetry in the same `otel-tui` session.

## Start `otel-tui`

Install it once:

```bash
go install github.com/ymtdzzz/otel-tui@latest
```

Then start it in its own terminal:

```bash
otel-tui
```

By default, `otel-tui` accepts:

- OTLP gRPC on `http://127.0.0.1:4317`
- OTLP HTTP on `http://127.0.0.1:4318`

The examples below use gRPC on `4317` so the same endpoint works for `imagine-tui`, Claude Code, and Codex.

## Build `imagine-tui`

From the repo root:

```bash
go build -o imagine-tui ./cmd/imagine-tui
```

If you prefer, you can replace `./imagine-tui` with `go run ./cmd/imagine-tui` in the commands below.

## Run `imagine-tui serve`

Open a second terminal. This is the TUI renderer terminal.

Set OTLP for the `serve` process:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

Then start the server on a Unix socket:

```bash
cd <repo-root>
./imagine-tui serve --socket /tmp/imagine-tui.sock
```

This terminal owns the actual TUI. The agent talks to it through `/tmp/imagine-tui.sock`.

## Run with Claude Code

The checked-in `demo/log-viewer/.mcp.json` already tells Claude Code to spawn:

```bash
go run ../../cmd/imagine-tui connect /tmp/imagine-tui.sock
```

Open a third terminal and set telemetry for both Claude Code and the `imagine-tui connect` subprocess it will launch:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_LOGS_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

Then run Claude Code inside the demo:

```bash
cd <repo-root>/demo/log-viewer
claude
```

Claude Code will discover `.mcp.json`, start `imagine-tui connect`, and talk to the renderer over the socket.

## Run with Codex

Codex uses `config.toml` instead of `.mcp.json`. The easiest local setup is a project-local config at `demo/log-viewer/.codex/config.toml`.

Create this file:

```toml
[otel]
environment = "local"
exporter = { otlp-grpc = { endpoint = "http://127.0.0.1:4317" } }
trace_exporter = { otlp-grpc = { endpoint = "http://127.0.0.1:4317" } }
metrics_exporter = { otlp-grpc = { endpoint = "http://127.0.0.1:4317" } }
# Optional: include raw prompt text in Codex OTel logs.
# log_user_prompt = true

[mcp_servers.imagine_tui]
command = "go"
args = ["run", "../../cmd/imagine-tui", "connect", "/tmp/imagine-tui.sock"]
env = { OTEL_EXPORTER_OTLP_ENDPOINT = "http://127.0.0.1:4317", OTEL_EXPORTER_OTLP_PROTOCOL = "grpc" }
```

Then run Codex in the same demo directory:

```bash
cd <repo-root>/demo/log-viewer
codex
```

Codex will start `imagine-tui connect` as an MCP subprocess, send its own telemetry to `otel-tui`, and the bridge process will do the same.

## Example prompt

Use the same prompt for either Claude Code or Codex:

```text
Build me a log viewer for the sample data in sample-data/app.log
```

As the agent works, the TUI should appear in the `imagine-tui serve` terminal and start updating as widgets are created and patched.

## What you should see in `otel-tui`

For `imagine-tui`, you should see trace spans such as:

- `command.serve`
- `serve.socket`
- `mcp.session.socket`
- `command.connect`
- `connect.dial`
- `mcp.tool.replace`
- `mcp.tool.await_event`
- `render.update`
- `render.refresh_scripts_and_widgets`
- `render.sync_state`
- `render.route_widget_event`

That span tree gives you the end-to-end path from the agent's MCP call to the UI mutation and render work.

For Codex, you should also see Codex log events and metrics in `otel-tui`, including events around conversation start, API requests, tool decisions, and tool results. If you leave `otel.log_user_prompt` at its default, the prompt content will be redacted in Codex logs.

For Claude Code, you should expect logs/events and metrics for the Claude session, plus the `imagine-tui connect` spans from the subprocess Claude launched. Claude data will correlate in the same OTLP sink, but it will not appear as one continuous distributed span tree because Claude Code does not currently emit spans.

## A good local mental model

When everything is working, you have four active processes:

1. `otel-tui`
2. `imagine-tui serve --socket /tmp/imagine-tui.sock`
3. `claude` or `codex`
4. `imagine-tui connect /tmp/imagine-tui.sock` started by the agent as an MCP subprocess

All four do different jobs. The most common reason telemetry looks incomplete is that only one or two of them were pointed at the OTLP receiver.

## If telemetry is missing

Check these first:

- `otel-tui` is running before you start the other processes
- `imagine-tui serve` was started with `OTEL_EXPORTER_OTLP_ENDPOINT` set
- the agent-side `imagine-tui connect` process inherited OTLP settings
- Claude Code has `CLAUDE_CODE_ENABLE_TELEMETRY=1`
- Codex has an `[otel]` block in `config.toml`
