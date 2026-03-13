package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAgentBuildsDemoAppOverBridgeAndTUIWorks(t *testing.T) {
	socketPath, ptmx, capture, cleanup := startServePTYTerminal(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	bridge := startConnectBridgeSession(t, ctx, socketPath)

	assertToolDiscovery(t, ctx, bridge.sess)

	widgets := mustCallTool(t, ctx, bridge.sess, "describe_widgets", nil)
	widgetText := toolResultText(t, widgets)
	for _, want := range []string{"button", "input", "log"} {
		if !strings.Contains(widgetText, want) {
			t.Fatalf("describe_widgets missing %q: %s", want, widgetText)
		}
	}

	scripting := mustCallTool(t, ctx, bridge.sess, "describe_scripting", nil)
	scriptingText := toolResultText(t, scripting)
	for _, want := range []string{"emit('agent', data)", "on_change", "computed props"} {
		if !strings.Contains(scriptingText, want) {
			t.Fatalf("describe_scripting missing %q: %s", want, scriptingText)
		}
	}

	layout := mustCallTool(t, ctx, bridge.sess, "layout", map[string]any{
		"tree": map[string]any{
			"id":   "mission",
			"type": "container",
			"props": map[string]any{
				"direction": "vertical",
				"padding":   1,
				"gap":       1,
			},
			"children": []any{
				map[string]any{
					"id":    "title",
					"type":  "text",
					"props": map[string]any{"content": "Mission Control", "style": "bold"},
				},
				map[string]any{
					"id":    "status",
					"type":  "text",
					"props": map[string]any{"content": "Status: idle"},
				},
				map[string]any{
					"id":   "command",
					"type": "input",
					"props": map[string]any{
						"placeholder": "Enter command",
					},
					"scripts": map[string]any{
						"on_change": "emit('local', [{op: 'update', id: 'status', props: {content: 'Status: preview ' + $.value}}]); emit('agent', {action: 'preview', value: $.value})",
					},
				},
				map[string]any{
					"id":       "count",
					"type":     "text",
					"computed": map[string]any{"content": "return 'Chars: ' + (($('command').value || '').length)"},
				},
				map[string]any{
					"id":    "send",
					"type":  "button",
					"props": map[string]any{"label": "Send"},
				},
				map[string]any{
					"id":   "feed",
					"type": "log",
					"props": map[string]any{
						"auto_scroll": true,
						"max_lines":   50,
					},
				},
			},
		},
	})
	if layout.IsError {
		t.Fatalf("layout returned error: %v", layout.Content)
	}

	appended := mustCallTool(t, ctx, bridge.sess, "append_items", map[string]any{
		"target": "feed",
		"items": []any{
			map[string]any{"text": "Mission Control online", "level": "debug"},
			map[string]any{"text": "Awaiting operator input"},
		},
	})
	if appended.IsError {
		t.Fatalf("append_items returned error: %v", appended.Content)
	}

	waitForOutput(t, capture, "Mission Control")
	waitForOutput(t, capture, "Status: idle")
	waitForOutput(t, capture, "Chars: 0")
	waitForOutput(t, capture, "Mission Control online")
	waitForOutput(t, capture, "Enter command")

	previewCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms": 3000,
				"filter":     []any{"command"},
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		previewCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("warp")); err != nil {
		t.Fatalf("write command to PTY: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("await_event preview failed: %v", err)
	case text := <-previewCh:
		for _, want := range []string{`"source":"command"`, `"action":"preview"`, `"value":"warp"`} {
			if !strings.Contains(text, want) {
				t.Fatalf("preview event missing %q: %s", want, text)
			}
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for preview event")
	}

	waitForOutput(t, capture, "Status: preview warp")
	waitForOutput(t, capture, "Chars: 4")
	waitForOutput(t, capture, "warp")

	clickCh := make(chan string, 1)
	go func() {
		res, err := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms": 3000,
				"filter":     []any{"send"},
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		clickCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("\t\r")); err != nil {
		t.Fatalf("write tab/enter to PTY: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("await_event click failed: %v", err)
	case text := <-clickCh:
		for _, want := range []string{`"source":"send"`, `"event":"click"`} {
			if !strings.Contains(text, want) {
				t.Fatalf("click event missing %q: %s", want, text)
			}
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for click event")
	}

	bridge.close(t)
	waitForOutput(t, capture, "Agent disconnected.")
	waitForOutput(t, capture, "Waiting for reconnect...")

	reconnected := startConnectBridgeSession(t, ctx, socketPath)
	defer reconnected.close(t)
	assertToolDiscovery(t, ctx, reconnected.sess)

	query := mustCallTool(t, ctx, reconnected.sess, "query", map[string]any{
		"ids": []any{"status", "command", "count", "send"},
	})
	queryText := toolResultText(t, query)
	for _, want := range []string{"Status: preview warp", `"value":"warp"`, "Chars: 4", "Send"} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("query after reconnect missing %q: %s", want, queryText)
		}
	}

	patched := mustCallTool(t, ctx, reconnected.sess, "patch", map[string]any{
		"ops": []any{
			map[string]any{
				"op":    "update",
				"id":    "status",
				"props": map[string]any{"content": "Status: launched warp"},
			},
			map[string]any{
				"op":    "update",
				"id":    "send",
				"props": map[string]any{"label": "Sent", "disabled": true},
			},
		},
	})
	if patched.IsError {
		t.Fatalf("patch returned error: %v", patched.Content)
	}

	appended = mustCallTool(t, ctx, reconnected.sess, "append_items", map[string]any{
		"target": "feed",
		"items": []any{
			map[string]any{"text": "Command accepted: warp", "level": "warn"},
		},
	})
	if appended.IsError {
		t.Fatalf("append_items follow-up returned error: %v", appended.Content)
	}

	waitForOutput(t, capture, "Status: launched warp")
	waitForOutput(t, capture, "Command accepted: warp")
	waitForOutput(t, capture, "Sent")
}

func TestAgentIteratesOnMultiStepDemoScenarioOverBridge(t *testing.T) {
	socketPath, ptmx, capture, cleanup := startServePTYTerminal(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	bridge := startConnectBridgeSession(t, ctx, socketPath)
	defer bridge.close(t)

	assertToolDiscovery(t, ctx, bridge.sess)

	widgets := mustCallTool(t, ctx, bridge.sess, "describe_widgets", nil)
	if text := toolResultText(t, widgets); !strings.Contains(text, "input") || !strings.Contains(text, "button") {
		t.Fatalf("unexpected widget discovery: %s", text)
	}
	scripting := mustCallTool(t, ctx, bridge.sess, "describe_scripting", nil)
	if text := toolResultText(t, scripting); !strings.Contains(text, "emit('agent', data)") {
		t.Fatalf("unexpected scripting discovery: %s", text)
	}

	layout := mustCallTool(t, ctx, bridge.sess, "layout", map[string]any{
		"tree": map[string]any{
			"id":   "console",
			"type": "container",
			"props": map[string]any{
				"direction": "vertical",
				"padding":   1,
				"gap":       1,
			},
			"children": []any{
				map[string]any{
					"id":    "phase",
					"type":  "text",
					"props": map[string]any{"content": "Phase 0: draft"},
				},
				map[string]any{
					"id":   "orders",
					"type": "input",
					"props": map[string]any{
						"placeholder": "Describe mission",
					},
					"scripts": map[string]any{
						"on_change": "emit('local', [{op: 'update', id: 'preview', props: {content: 'Preview: ' + $.value}}, {op: 'update', id: 'action', props: {label: $.value ? 'Commit Plan' : 'Draft Plan'}}]); emit('agent', {action: 'preview', value: $.value})",
					},
				},
				map[string]any{
					"id":    "preview",
					"type":  "text",
					"props": map[string]any{"content": "Preview: "},
				},
				map[string]any{
					"id":    "action",
					"type":  "button",
					"props": map[string]any{"label": "Draft Plan"},
				},
				map[string]any{
					"id":   "timeline",
					"type": "log",
					"props": map[string]any{
						"auto_scroll": true,
						"max_lines":   2,
					},
				},
			},
		},
	})
	if layout.IsError {
		t.Fatalf("layout returned error: %v", layout.Content)
	}

	appended := mustCallTool(t, ctx, bridge.sess, "append_items", map[string]any{
		"target": "timeline",
		"items": []any{
			map[string]any{"text": "Planning console online", "level": "debug"},
			map[string]any{"text": "Awaiting mission draft"},
		},
	})
	if appended.IsError {
		t.Fatalf("append_items returned error: %v", appended.Content)
	}

	waitForOutput(t, capture, "Phase 0: draft")
	waitForOutput(t, capture, "Describe mission")
	waitForOutput(t, capture, "Preview:")
	waitForOutput(t, capture, "Planning console online")

	previewCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms":  3000,
				"filter":      []any{"orders"},
				"debounce_ms": 100,
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		previewCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("survey delta rift")); err != nil {
		t.Fatalf("write orders to PTY: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("await_event preview failed: %v", err)
	case text := <-previewCh:
		for _, want := range []string{`"source":"orders"`, `"action":"preview"`, `"value":"survey delta rift"`} {
			if !strings.Contains(text, want) {
				t.Fatalf("preview event missing %q: %s", want, text)
			}
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for preview event")
	}

	waitForOutput(t, capture, "Preview: survey delta rift")
	waitForOutput(t, capture, "Commit Plan")

	patched := mustCallTool(t, ctx, bridge.sess, "patch", map[string]any{
		"ops": []any{
			map[string]any{
				"op":    "update",
				"id":    "phase",
				"props": map[string]any{"content": "Phase 1: ready to commit"},
			},
		},
	})
	if patched.IsError {
		t.Fatalf("patch after preview returned error: %v", patched.Content)
	}
	appended = mustCallTool(t, ctx, bridge.sess, "append_items", map[string]any{
		"target": "timeline",
		"items": []any{
			map[string]any{"text": "Plan drafted from operator input", "level": "debug"},
		},
	})
	if appended.IsError {
		t.Fatalf("append_items after preview returned error: %v", appended.Content)
	}

	waitForOutput(t, capture, "Phase 1: ready to commit")
	waitForOutput(t, capture, "Plan drafted from operator input")

	actionCh := make(chan string, 1)
	go func() {
		res, err := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms": 3000,
				"filter":     []any{"action"},
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		actionCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("\t\r")); err != nil {
		t.Fatalf("write tab/enter to PTY: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("await_event action failed: %v", err)
	case text := <-actionCh:
		for _, want := range []string{`"source":"action"`, `"event":"click"`, `"orders":{"value":"survey delta rift"}`} {
			if !strings.Contains(text, want) {
				t.Fatalf("action event missing %q: %s", want, text)
			}
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for action click")
	}

	patched = mustCallTool(t, ctx, bridge.sess, "patch", map[string]any{
		"ops": []any{
			map[string]any{
				"op":    "update",
				"id":    "phase",
				"props": map[string]any{"content": "Phase 2: plan committed"},
			},
			map[string]any{
				"op":        "insert",
				"parent_id": "console",
				"index":     4,
				"node": map[string]any{
					"id":    "launch",
					"type":  "button",
					"props": map[string]any{"label": "Launch Drone"},
				},
			},
		},
	})
	if patched.IsError {
		t.Fatalf("patch after action returned error: %v", patched.Content)
	}
	appended = mustCallTool(t, ctx, bridge.sess, "append_items", map[string]any{
		"target": "timeline",
		"items": []any{
			map[string]any{"text": "Plan committed. Drone ready.", "level": "warn"},
		},
	})
	if appended.IsError {
		t.Fatalf("append_items after action returned error: %v", appended.Content)
	}

	waitForOutput(t, capture, "Phase 2: plan committed")
	waitForOutput(t, capture, "Launch Drone")
	waitForOutput(t, capture, "Plan committed. Drone ready.")

	launchCh := make(chan string, 1)
	go func() {
		res, err := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms": 3000,
				"filter":     []any{"launch"},
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		launchCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("\t\r")); err != nil {
		t.Fatalf("write tab/enter to PTY for launch: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("await_event launch failed: %v", err)
	case text := <-launchCh:
		for _, want := range []string{`"source":"launch"`, `"event":"click"`} {
			if !strings.Contains(text, want) {
				t.Fatalf("launch event missing %q: %s", want, text)
			}
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for launch click")
	}

	patched = mustCallTool(t, ctx, bridge.sess, "patch", map[string]any{
		"ops": []any{
			map[string]any{
				"op":    "update",
				"id":    "phase",
				"props": map[string]any{"content": "Phase 3: drone launched"},
			},
			map[string]any{
				"op":    "update",
				"id":    "launch",
				"props": map[string]any{"label": "Drone Deployed", "disabled": true},
			},
		},
	})
	if patched.IsError {
		t.Fatalf("patch after launch returned error: %v", patched.Content)
	}
	appended = mustCallTool(t, ctx, bridge.sess, "append_items", map[string]any{
		"target": "timeline",
		"items": []any{
			map[string]any{"text": "Drone launched into delta rift", "level": "warn"},
		},
	})
	if appended.IsError {
		t.Fatalf("append_items after launch returned error: %v", appended.Content)
	}

	waitForOutput(t, capture, "Phase 3: drone launched")
	waitForOutput(t, capture, "Drone Deployed")
	waitForOutput(t, capture, "Drone launched into delta rift")

	query := mustCallTool(t, ctx, bridge.sess, "query", map[string]any{
		"ids": []any{"phase", "preview", "launch"},
	})
	queryText := toolResultText(t, query)
	for _, want := range []string{"Phase 3: drone launched", "Preview: survey delta rift", "Drone Deployed"} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("final query missing %q: %s", want, queryText)
		}
	}
}

func mustCallTool(t *testing.T, ctx context.Context, sess *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult { //nolint:revive // ctx after t is intentional for test helpers
	t.Helper()

	params := &mcp.CallToolParams{Name: name}
	if args != nil {
		params.Arguments = args
	}
	res, err := sess.CallTool(ctx, params)
	if err != nil {
		t.Fatalf("%s failed: %v", name, err)
	}
	return res
}
