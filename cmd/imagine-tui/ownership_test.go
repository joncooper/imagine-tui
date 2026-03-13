package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

var builtTestBinary struct {
	once sync.Once
	path string
	err  error
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestOwnerGateSingleActive(t *testing.T) {
	var gate ownerGate

	first, ok := gate.TryClaim()
	if !ok || first != 1 {
		t.Fatalf("first claim = (%d, %v), want (1, true)", first, ok)
	}
	if _, ok := gate.TryClaim(); ok {
		t.Fatal("second claim should be rejected while owner is active")
	}
	if !gate.Release(first) {
		t.Fatal("expected active owner release to succeed")
	}

	second, ok := gate.TryClaim()
	if !ok || second != 2 {
		t.Fatalf("second claim after release = (%d, %v), want (2, true)", second, ok)
	}
}

func TestOwnerGateRejectsStaleRelease(t *testing.T) {
	var gate ownerGate

	first, ok := gate.TryClaim()
	if !ok {
		t.Fatal("expected first claim to succeed")
	}
	if gate.Release(first + 1) {
		t.Fatal("stale release should fail")
	}
	if !gate.Release(first) {
		t.Fatal("current owner release should succeed")
	}
}

func TestShellOwnedSessionReconnectsWithoutBlankingThePTY(t *testing.T) {
	socketPath, capture, cleanup := startServePTY(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn1, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	client1 := mcp.NewClient(&mcp.Implementation{Name: "client-1", Version: "0.0.1"}, nil)
	session1, err := client1.Connect(ctx, &mcp.IOTransport{Reader: conn1, Writer: conn1}, nil)
	if err != nil {
		_ = conn1.Close()
		t.Fatal(err)
	}

	result, err := session1.CallTool(ctx, &mcp.CallToolParams{
		Name: "replace",
		Arguments: map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"children": []any{
					map[string]any{
						"id":    "header",
						"type":  "text",
						"props": map[string]any{"text": "Hello PTY"},
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

	waitForOutput(t, capture, "Hello PTY")
	_ = session1.Close()
	_ = conn1.Close()
	waitForOutput(t, capture, "Agent disconnected.")
	waitForOutput(t, capture, "reconnect...")

	conn2, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn2.Close() }()
	client2 := mcp.NewClient(&mcp.Implementation{Name: "client-2", Version: "0.0.1"}, nil)
	session2, err := client2.Connect(ctx, &mcp.IOTransport{Reader: conn2, Writer: conn2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session2.Close() }()

	query, err := session2.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query",
		Arguments: map[string]any{"ids": []any{"header"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.IsError {
		t.Fatalf("query returned error after reconnect: %v", query.Content)
	}
}

func TestActiveOwnerRejectsSecondSocketClient(t *testing.T) {
	socketPath, _, cleanup := startServePTY(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn1, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn1.Close() }()
	client1 := mcp.NewClient(&mcp.Implementation{Name: "client-1", Version: "0.0.1"}, nil)
	session1, err := client1.Connect(ctx, &mcp.IOTransport{Reader: conn1, Writer: conn1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session1.Close() }()

	secondConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = secondConn.Close() }()

	if err := secondConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := secondConn.Read(buf); err == nil {
		t.Fatal("expected rejected second client connection to close")
	}

	tools, err := session1.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("active owner should remain usable: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("expected tools from active owner session")
	}
}

func TestServePTYEndToEndRendersAndAcceptsInput(t *testing.T) {
	socketPath, ptmx, capture, cleanup := startServePTYTerminal(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: conn, Writer: conn}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	waitForOutput(t, capture, "\x1b[?1049h")

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
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
						"props": map[string]any{"text": "TTY E2E"},
					},
					map[string]any{
						"id":    "cmd",
						"type":  "input",
						"props": map[string]any{"placeholder": "Type command"},
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

	waitForOutput(t, capture, "TTY E2E")
	waitForOutput(t, capture, "Type command")

	changeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		res, callErr := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"filter":      []any{"cmd"},
				"timeout_ms":  2000,
				"debounce_ms": 50,
			},
		})
		if callErr != nil {
			errCh <- callErr
			return
		}
		changeCh <- toolResultText(t, res)
	}()

	if _, err := ptmx.Write([]byte("go")); err != nil {
		t.Fatalf("write input to PTY: %v", err)
	}

	select {
	case callErr := <-errCh:
		t.Fatalf("await_event for input change failed: %v", callErr)
	case text := <-changeCh:
		if !strings.Contains(text, "\"event\":\"change\"") {
			t.Fatalf("expected change event, got: %s", text)
		}
		if !strings.Contains(text, "\"source\":\"cmd\"") {
			t.Fatalf("expected cmd source, got: %s", text)
		}
		if !strings.Contains(text, "\"value\":\"go\"") {
			t.Fatalf("expected input value go, got: %s", text)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for input change event")
	}

	waitForOutput(t, capture, "go")

	clickCh := make(chan string, 1)
	go func() {
		res, callErr := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "await_event",
			Arguments: map[string]any{
				"filter":     []any{"run"},
				"timeout_ms": 2000,
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
	case callErr := <-errCh:
		t.Fatalf("await_event for button click failed: %v", callErr)
	case text := <-clickCh:
		if !strings.Contains(text, "\"event\":\"click\"") {
			t.Fatalf("expected click event, got: %s", text)
		}
		if !strings.Contains(text, "\"source\":\"run\"") {
			t.Fatalf("expected run source, got: %s", text)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for button click event")
	}
}

func startServePTY(t *testing.T) (string, *threadSafeBuffer, func()) {
	socketPath, _, capture, cleanup := startServePTYTerminal(t)
	return socketPath, capture, cleanup
}

func startServePTYTerminal(t *testing.T) (string, *os.File, *threadSafeBuffer, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("/tmp", "imtu-*.sock")
	if err != nil {
		t.Fatalf("create temp socket path: %v", err)
	}
	socketPath := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(socketPath)

	cmd := exec.Command(testBinaryPath(t), "serve", "-socket", socketPath)
	cmd.Dir = "."

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}

	capture := &threadSafeBuffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(capture, ptmx)
		close(done)
	}()

	waitForOutput(t, capture, "Listening on")

	cleanup := func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		<-done
		_ = os.Remove(socketPath)
	}

	return socketPath, ptmx, capture, cleanup
}

func testBinaryPath(t *testing.T) string {
	t.Helper()

	builtTestBinary.once.Do(func() {
		tmpFile, err := os.CreateTemp("/tmp", "imagine-tui-test-*")
		if err != nil {
			builtTestBinary.err = fmt.Errorf("create temp binary path: %w", err)
			return
		}
		builtTestBinary.path = tmpFile.Name()
		_ = tmpFile.Close()
		_ = os.Remove(builtTestBinary.path)

		cmd := exec.Command("go", "build", "-o", builtTestBinary.path, ".")
		cmd.Dir = "."
		output, err := cmd.CombinedOutput()
		if err != nil {
			builtTestBinary.err = fmt.Errorf("build test binary: %w: %s", err, strings.TrimSpace(string(output)))
		}
	})

	if builtTestBinary.err != nil {
		t.Fatal(builtTestBinary.err)
	}
	return builtTestBinary.path
}

func waitForOutput(t *testing.T, capture *threadSafeBuffer, needle string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(capture.String(), needle) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in PTY output:\n%s", needle, capture.String())
}

func toolResultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("result has no content")
	}
	data, err := json.Marshal(r.Content[0])
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	var wire struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	return wire.Text
}
