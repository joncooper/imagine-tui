package render

import (
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
)

// mockProgram records messages sent to it.
type mockProgram struct {
	msgs []tea.Msg
}

func (m *mockProgram) Send(msg tea.Msg) {
	m.msgs = append(m.msgs, msg)
}

func TestBridgeNotifiesOnDOMChange(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	mp := &mockProgram{}
	bridge := NewBridge(srv, mp)

	bridge.NotifyDOMChanged()

	if len(mp.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mp.msgs))
	}
	if _, ok := mp.msgs[0].(DOMChangedMsg); !ok {
		t.Errorf("expected DOMChangedMsg, got %T", mp.msgs[0])
	}
}

func TestBridgeNotifiesDisconnect(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	mp := &mockProgram{}
	bridge := NewBridge(srv, mp)

	bridge.NotifyDisconnected(7, nil)

	if len(mp.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mp.msgs))
	}
	msg, ok := mp.msgs[0].(MCPDisconnectedMsg)
	if !ok {
		t.Errorf("expected MCPDisconnectedMsg, got %T", mp.msgs[0])
	}
	if msg.SessionID != 7 {
		t.Errorf("SessionID = %d, want 7", msg.SessionID)
	}
}

func TestBridgeNotifiesConnect(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	mp := &mockProgram{}
	bridge := NewBridge(srv, mp)

	bridge.NotifyConnected(9)

	if len(mp.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mp.msgs))
	}
	msg, ok := mp.msgs[0].(MCPConnectedMsg)
	if !ok {
		t.Fatalf("expected MCPConnectedMsg, got %T", mp.msgs[0])
	}
	if msg.SessionID != 9 {
		t.Errorf("SessionID = %d, want 9", msg.SessionID)
	}
}

func TestBridgeServer(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	mp := &mockProgram{}
	bridge := NewBridge(srv, mp)

	if bridge.Server() != srv {
		t.Error("Server() should return the wrapped MCP server")
	}
}

func TestBridgeOnMutationCallback(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	mp := &mockProgram{}
	bridge := NewBridge(srv, mp)

	var called atomic.Int32
	bridge.OnMutation = func() { called.Add(1) }

	bridge.NotifyDOMChanged()

	if called.Load() != 1 {
		t.Errorf("OnMutation called %d times, want 1", called.Load())
	}
	if len(mp.msgs) != 1 {
		t.Errorf("expected 1 msg, got %d", len(mp.msgs))
	}
}
