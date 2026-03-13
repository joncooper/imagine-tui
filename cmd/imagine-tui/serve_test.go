package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

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
	srv.SetOnMutation(func() {
		mutationCount.Add(1)
		bridge.NotifyDOMChanged()
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
