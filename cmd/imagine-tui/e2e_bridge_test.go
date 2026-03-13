package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bridgeSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *bytes.Buffer
	sess   *mcp.ClientSession
}

func TestConnectBridgeReconnectListsTools(t *testing.T) {
	socketPath, _, _, cleanup := startServePTYTerminal(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first := startConnectBridgeSession(t, ctx, socketPath)
	assertToolDiscovery(t, ctx, first.sess)
	first.close(t)

	second := startConnectBridgeSession(t, ctx, socketPath)
	defer second.close(t)
	assertToolDiscovery(t, ctx, second.sess)
}

func TestConnectBridgeServePTYEndToEndInputAndScripting(t *testing.T) {
	socketPath, ptmx, capture, cleanup := startServePTYTerminal(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bridge := startConnectBridgeSession(t, ctx, socketPath)
	defer bridge.close(t)
	assertToolDiscovery(t, ctx, bridge.sess)

	result, err := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
		Name: "replace",
		Arguments: map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"props": map[string]any{
					"direction": "vertical",
				},
				"children": []any{
					map[string]any{
						"id":    "title",
						"type":  "text",
						"props": map[string]any{"text": "Bridge E2E"},
					},
					map[string]any{
						"id":    "status",
						"type":  "text",
						"props": map[string]any{"text": "Idle"},
					},
					map[string]any{
						"id":    "cmd",
						"type":  "input",
						"props": map[string]any{"placeholder": "Type command"},
						"scripts": map[string]any{
							"on_change": "emit('local', [{op: 'update', id: 'status', props: {text: 'typed: ' + $.value}}]); emit('agent', {action: 'typed', value: $.value})",
						},
					},
					map[string]any{
						"id":    "run",
						"type":  "button",
						"props": map[string]any{"label": "Run"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("replace returned error: %v", result.Content)
	}

	waitForOutput(t, capture, "Bridge E2E")
	waitForOutput(t, capture, "Type command")

	typedCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		res, callErr := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms": 2000,
				"filter":     []any{"cmd"},
			},
		})
		if callErr != nil {
			errCh <- callErr
			return
		}
		typedCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("go")); err != nil {
		t.Fatalf("write input to PTY: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("await_event failed: %v", err)
	case text := <-typedCh:
		if !strings.Contains(text, "\"action\":\"typed\"") {
			t.Fatalf("expected typed action, got: %s", text)
		}
		if !strings.Contains(text, "\"source\":\"cmd\"") {
			t.Fatalf("expected cmd source, got: %s", text)
		}
		if !strings.Contains(text, "\"value\":\"go\"") {
			t.Fatalf("expected input value go, got: %s", text)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for typed event")
	}

	waitForOutput(t, capture, "typed: go")

	clickCh := make(chan string, 1)
	go func() {
		res, callErr := bridge.sess.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"timeout_ms": 2000,
				"filter":     []any{"run"},
			},
		})
		if callErr != nil {
			errCh <- callErr
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
		if !strings.Contains(text, "\"event\":\"click\"") {
			t.Fatalf("expected click event, got: %s", text)
		}
		if !strings.Contains(text, "\"source\":\"run\"") {
			t.Fatalf("expected run source, got: %s", text)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for click event")
	}
}

func TestConnectBridgeReconnectPreservesLiveUIState(t *testing.T) {
	socketPath, _, capture, cleanup := startServePTYTerminal(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first := startConnectBridgeSession(t, ctx, socketPath)
	result, err := first.sess.CallTool(ctx, &mcp.CallToolParams{
		Name: "replace",
		Arguments: map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"children": []any{
					map[string]any{
						"id":    "header",
						"type":  "text",
						"props": map[string]any{"text": "Persisted UI"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("replace returned error: %v", result.Content)
	}
	waitForOutput(t, capture, "Persisted UI")
	first.close(t)

	second := startConnectBridgeSession(t, ctx, socketPath)
	defer second.close(t)
	assertToolDiscovery(t, ctx, second.sess)

	query, err := second.sess.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query",
		Arguments: map[string]any{"ids": []any{"header"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.IsError {
		t.Fatalf("query returned error after reconnect: %v", query.Content)
	}

	text := toolResultText(t, query)
	if !strings.Contains(text, "Persisted UI") {
		t.Fatalf("expected persisted UI after reconnect, got: %s", text)
	}
}

func startConnectBridgeSession(t *testing.T, parentCtx context.Context, socketPath string) *bridgeSession {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	var lastStderr string
	for time.Now().Before(deadline) {
		attemptCtx, cancel := context.WithTimeout(parentCtx, 750*time.Millisecond)
		bs, err := tryStartConnectBridgeSession(t, attemptCtx, socketPath)
		cancel()
		if err == nil {
			return bs
		}
		lastErr = err
		if bridgeErr, ok := err.(*bridgeStartError); ok {
			lastStderr = bridgeErr.stderr
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastStderr != "" {
		t.Fatalf("failed to start connect bridge session: %v\nstderr:\n%s", lastErr, lastStderr)
	}
	t.Fatalf("failed to start connect bridge session: %v", lastErr)
	return nil
}

type bridgeStartError struct {
	err    error
	stderr string
}

func (e *bridgeStartError) Error() string { return e.err.Error() }

func tryStartConnectBridgeSession(t *testing.T, ctx context.Context, socketPath string) (*bridgeSession, error) {
	t.Helper()

	cmd := exec.Command(testBinaryPath(t), "connect", socketPath)
	cmd.Dir = "."
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "bridge-client", Version: "0.0.1"}, nil)
	sess, err := client.Connect(ctx, &mcp.IOTransport{Reader: stdout, Writer: stdin}, nil)
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, &bridgeStartError{err: err, stderr: stderr.String()}
	}

	return &bridgeSession{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr, sess: sess}, nil
}

func (bs *bridgeSession) close(t *testing.T) {
	t.Helper()

	if bs.sess != nil {
		_ = bs.sess.Close()
	}
	if bs.stdin != nil {
		_ = bs.stdin.Close()
	}
	if bs.stdout != nil {
		_ = bs.stdout.Close()
	}
	if bs.cmd != nil && bs.cmd.Process != nil {
		_ = bs.cmd.Process.Kill()
		_, _ = bs.cmd.Process.Wait()
	}
}

func assertToolDiscovery(t *testing.T, ctx context.Context, sess *mcp.ClientSession) {
	t.Helper()

	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools.Tools) < 12 {
		names := make([]string, len(tools.Tools))
		for i, tool := range tools.Tools {
			names[i] = tool.Name
		}
		t.Fatalf("expected at least 12 tools, got %d: %v", len(tools.Tools), names)
	}
	for _, want := range []string{"describe_widgets", "describe_scripting", "replace", "layout", "set_items", "await_event", "patch"} {
		if !hasToolNamed(tools.Tools, want) {
			t.Fatalf("missing tool %q from discovery", want)
		}
	}
}

func hasToolNamed(tools []*mcp.Tool, want string) bool {
	for _, tool := range tools {
		if tool.Name == want {
			return true
		}
	}
	return false
}
