package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/creack/pty"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/render"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	tea "github.com/charmbracelet/bubbletea"
)

// mockProgram captures messages sent from the bridge.
type mockProgram struct {
	msgs []tea.Msg
}

func (m *mockProgram) Send(msg tea.Msg) {
	m.msgs = append(m.msgs, msg)
}

func TestUnixSocketIntegration(t *testing.T) {
	// Create server.
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	// Wire mutation callback through the bridge.
	mp := &mockProgram{}
	bridge := render.NewBridge(srv, mp)
	var mutationCount atomic.Int32
	srv.SetOnMutation(func(kind imcp.MutationKind) {
		mutationCount.Add(1)
		bridge.NotifyDOMChanged(kind)
	})

	// Listen on a temp Unix socket.
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Accept one connection.
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		connCh <- conn
	}()

	// Dial the socket from the client side.
	clientConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = clientConn.Close() }()

	// Server side: wrap connection in IOTransport and connect.
	serverConn := <-connCh
	defer func() { _ = serverConn.Close() }()

	serverTransport := &mcp.IOTransport{
		Reader: serverConn,
		Writer: serverConn,
	}
	_, err = srv.MCPServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	// Client side: wrap connection in IOTransport and connect.
	clientTransport := &mcp.IOTransport{
		Reader: clientConn,
		Writer: clientConn,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// ListTools — verify all 6 tools are present.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 12 {
		names := make([]string, len(tools.Tools))
		for i, tool := range tools.Tools {
			names[i] = tool.Name
		}
		t.Fatalf("expected 12 tools, got %d: %v", len(tools.Tools), names)
	}

	// Call replace to set up a tree.
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "replace",
		Arguments: map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"children": []any{
					map[string]any{
						"id":    "header",
						"type":  "text",
						"props": map[string]any{"content": "Hello from socket"},
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

	// Verify mutation callback fired.
	if mutationCount.Load() != 1 {
		t.Fatalf("expected 1 mutation, got %d", mutationCount.Load())
	}

	// Verify bridge sent DOMChangedMsg.
	if len(mp.msgs) == 0 {
		t.Fatal("expected DOMChangedMsg, got none")
	}
	if _, ok := mp.msgs[0].(render.DOMChangedMsg); !ok {
		t.Fatalf("expected DOMChangedMsg, got %T", mp.msgs[0])
	}

	// Call query to verify state.
	qResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query",
		Arguments: map[string]any{"ids": []any{"header"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if qResult.IsError {
		t.Fatalf("query returned error: %v", qResult.Content)
	}
}

// TestBridgedConnection simulates the exact flow Claude Code uses:
// MCP client → pipes → connect bridge (io.Copy) → Unix socket → server.
// This catches framing, buffering, or protocol issues in the bridge path.
func TestBridgedConnection(t *testing.T) {
	// Create server.
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	mp := &mockProgram{}
	bridge := render.NewBridge(srv, mp)
	srv.SetOnMutation(bridge.NotifyDOMChanged)

	// Listen on a temp Unix socket.
	socketPath := filepath.Join(t.TempDir(), "bridge.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Accept connections and wire them to the MCP server (like serveSocket does).
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				transport := &mcp.IOTransport{Reader: conn, Writer: conn}
				session, err := srv.MCPServer().Connect(ctx, transport, nil)
				if err != nil {
					return
				}
				bridge.NotifyConnected(1)
				_ = session.Wait()
			}()
		}
	}()

	// Create pipes to simulate what Claude Code sees:
	// Claude writes to clientWrite → bridge reads from bridgeStdin
	// Bridge writes to bridgeStdout → Claude reads from clientRead
	bridgeStdin, clientWrite := io.Pipe()
	clientRead, bridgeStdout := io.Pipe()

	// Run the connect bridge in a goroutine (simulates the connect subprocess).
	bridgeDone := make(chan error, 1)
	go func() {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			bridgeDone <- err
			return
		}
		defer func() { _ = conn.Close() }()

		errCh := make(chan error, 2)
		go func() {
			_, err := io.Copy(conn, bridgeStdin)
			errCh <- err
		}()
		go func() {
			_, err := io.Copy(bridgeStdout, conn)
			errCh <- err
		}()
		bridgeDone <- <-errCh
	}()

	// Claude Code side: wrap the pipes as an MCP client using IOTransport.
	clientTransport := &mcp.IOTransport{
		Reader: clientRead,
		Writer: clientWrite,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "claude-code", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect through bridge: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// ListTools — this is what Claude Code does to discover available tools.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools through bridge: %v", err)
	}

	if len(tools.Tools) != 12 {
		names := make([]string, len(tools.Tools))
		for i, tool := range tools.Tools {
			names[i] = tool.Name
		}
		t.Fatalf("expected 12 tools through bridge, got %d: %v", len(tools.Tools), names)
	}

	// Verify we can also call a tool through the bridge.
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "describe_widgets",
	})
	if err != nil {
		t.Fatalf("CallTool through bridge: %v", err)
	}
	if result.IsError {
		t.Fatalf("describe_widgets returned error through bridge")
	}

	t.Logf("Bridge test passed: %d tools discovered, describe_widgets callable", len(tools.Tools))
}

// TestFullStackPTY is a true end-to-end test: builds the binary, starts it
// with a real pty (so BubbleTea initializes), connects an MCP client through
// the socket, discovers tools, and builds a UI. This exercises the exact path
// that Claude Code uses in production.
func TestFullStackPTY(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full-stack pty test in short mode")
	}

	// Build the binary.
	binPath := filepath.Join(t.TempDir(), "imagine-tui")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	socketPath := filepath.Join(t.TempDir(), "full-stack.sock")
	logPath := filepath.Join(t.TempDir(), "server.log")

	// Start the server with a pty so BubbleTea can initialize.
	cmd := exec.Command(binPath, "serve", "-socket", socketPath, "-log", logPath)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	// Wait for the socket to appear (server is ready).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(socketPath); err != nil {
		// Dump the log for debugging.
		logData, _ := os.ReadFile(logPath)
		t.Fatalf("socket never appeared at %s\nserver log:\n%s", socketPath, logData)
	}

	t.Log("Server started, socket ready")

	// Connect an MCP client through the socket (same as Claude Code's connect bridge).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	clientTransport := &mcp.IOTransport{Reader: conn, Writer: conn}
	client := mcp.NewClient(&mcp.Implementation{Name: "claude-code-test", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		logData, _ := os.ReadFile(logPath)
		t.Fatalf("client connect: %v\nserver log:\n%s", err, logData)
	}
	defer func() { _ = cs.Close() }()

	// 1. List tools — the critical operation that's failing in production.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	toolNames := make([]string, len(tools.Tools))
	for i, tool := range tools.Tools {
		toolNames[i] = tool.Name
	}
	t.Logf("Discovered %d tools: %v", len(tools.Tools), toolNames)

	if len(tools.Tools) != 12 {
		t.Fatalf("expected 12 tools, got %d: %v", len(tools.Tools), toolNames)
	}

	// 2. Call describe_widgets.
	dwResult, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "describe_widgets"})
	if err != nil {
		t.Fatalf("describe_widgets: %v", err)
	}
	if dwResult.IsError {
		t.Fatalf("describe_widgets error: %v", dwResult.Content)
	}
	t.Log("describe_widgets: OK")

	// 3. Build a UI with layout.
	layoutResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "layout",
		Arguments: map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"props": map[string]any{
					"direction": "vertical",
					"padding":   1,
				},
				"children": []any{
					map[string]any{
						"id":    "title",
						"type":  "text",
						"props": map[string]any{"content": "Full Stack PTY Test", "style": "bold"},
					},
					map[string]any{
						"id":   "main-list",
						"type": "list",
						"props": map[string]any{
							"title": "Test Items",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	if layoutResult.IsError {
		t.Fatalf("layout error: %v", layoutResult.Content)
	}
	t.Log("layout: OK")

	// 4. Populate the list with set_items.
	setResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "set_items",
		Arguments: map[string]any{
			"target": "main-list",
			"items": []any{
				map[string]any{"id": "item-1", "label": "First item"},
				map[string]any{"id": "item-2", "label": "Second item"},
				map[string]any{"id": "item-3", "label": "Third item"},
			},
		},
	})
	if err != nil {
		t.Fatalf("set_items: %v", err)
	}
	if setResult.IsError {
		t.Fatalf("set_items error: %v", setResult.Content)
	}
	t.Log("set_items: OK")

	// 5. Query to verify DOM state.
	qResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query",
		Arguments: map[string]any{"ids": []any{"title", "main-list"}},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if qResult.IsError {
		t.Fatalf("query error: %v", qResult.Content)
	}

	// Parse and verify the query response.
	var queryResp struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	if len(qResult.Content) > 0 {
		if tc, ok := qResult.Content[0].(*mcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &queryResp); err != nil {
				t.Fatalf("parse query response: %v", err)
			}
			if _, ok := queryResp.Results["title"]; !ok {
				t.Fatal("query missing 'title' node")
			}
			if _, ok := queryResp.Results["main-list"]; !ok {
				t.Fatal("query missing 'main-list' node")
			}
		}
	}

	t.Log("Full stack PTY test passed: server started with BubbleTea, 12 tools discovered, UI built and verified")
}
