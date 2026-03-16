package render

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/widget"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- M5-3: MCP ↔ BubbleTea bridge integration tests ---

// callTool invokes a tool handler directly on the MCP server.
func callTool(t *testing.T, srv *imcp.Server, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := srv.CallTool(context.Background(), tool, args)
	if err != nil {
		t.Fatalf("CallTool %q: %v", tool, err)
	}
	return result
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
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

func TestMCPPatchTriggersReRender(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	// Call replace to set up a tree with a text node.
	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":    "msg",
					"type":  "text",
					"props": map[string]any{"text": "Hello World"},
				},
			},
		},
	})

	// Simulate DOMChangedMsg (as bridge would send).
	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	view := model.View()
	if !strings.Contains(view, "Hello World") {
		t.Errorf("after MCP replace, view should contain 'Hello World', got:\n%s", view)
	}
}

func TestMCPPatchUpdatesExistingNode(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	// Set up initial tree.
	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":    "msg",
					"type":  "text",
					"props": map[string]any{"text": "Before"},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	// Patch the text.
	callTool(t, srv, "patch", map[string]any{
		"ops": []map[string]any{
			{
				"op":    "update",
				"id":    "msg",
				"props": map[string]any{"text": "After"},
			},
		},
	})

	newM2, _ := model.Update(DOMChangedMsg{})
	model2 := newM2.(Model)

	view := model2.View()
	if !strings.Contains(view, "After") {
		t.Errorf("after patch, view should contain 'After', got:\n%s", view)
	}
	if strings.Contains(view, "Before") {
		t.Errorf("after patch, view should not contain 'Before', got:\n%s", view)
	}
}

func TestMCPAwaitEventBlocksAndReturns(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	// Set up tree with a button.
	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":    "btn",
					"type":  "button",
					"props": map[string]any{"label": "Click Me"},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	// Start await_event in a goroutine with timeout.
	var result *mcp.CallToolResult
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result = callTool(t, srv, "await_event", map[string]any{
			"timeout_ms": 2000,
		})
	}()

	// Give await_event time to start blocking.
	time.Sleep(50 * time.Millisecond)

	// Simulate button click by focusing and pressing Enter.
	model.focusedID = "btn"
	model.syncState()
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = newM2

	// Wait for await_event to return.
	wg.Wait()

	if result == nil {
		t.Fatal("await_event returned nil result")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "click") {
		t.Errorf("expected click event, got: %s", text)
	}
}

func TestMCPAwaitEventTimeout(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	result := callTool(t, srv, "await_event", map[string]any{
		"timeout_ms": 50,
	})

	text := resultText(t, result)
	if !strings.Contains(text, "timeout") {
		t.Errorf("expected timeout, got: %s", text)
	}
}

func TestConcurrentPatchAndKeypress(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	// Set up tree with input and text.
	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":    "input1",
					"type":  "input",
					"props": map[string]any{"placeholder": "Type here"},
				},
				{
					"id":    "status",
					"type":  "text",
					"props": map[string]any{"text": "Ready"},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)
	model.focusedID = "input1"

	// Run concurrent patch and keypress.
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		callTool(t, srv, "patch", map[string]any{
			"ops": []map[string]any{
				{
					"op":    "update",
					"id":    "status",
					"props": map[string]any{"text": "Processing..."},
				},
			},
		})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	}()

	wg.Wait()
	// Verify no panics or deadlocks.
}

func TestMCPSnapshotRestoreTriggersDOMChanged(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":    "msg",
					"type":  "text",
					"props": map[string]any{"text": "State A"},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	callTool(t, srv, "snapshot", map[string]any{"name": "checkpoint"})

	callTool(t, srv, "patch", map[string]any{
		"ops": []map[string]any{
			{"op": "update", "id": "msg", "props": map[string]any{"text": "State B"}},
		},
	})

	newM2, _ := model.Update(DOMChangedMsg{})
	model2 := newM2.(Model)
	view := model2.View()
	if !strings.Contains(view, "State B") {
		t.Errorf("expected State B, got:\n%s", view)
	}

	callTool(t, srv, "restore", map[string]any{"name": "checkpoint"})

	newM3, _ := model2.Update(DOMChangedMsg{})
	model3 := newM3.(Model)
	view = model3.View()
	if !strings.Contains(view, "State A") {
		t.Errorf("expected State A after restore, got:\n%s", view)
	}
}

func TestMCPQueryReturnsCurrentState(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":    "name",
					"type":  "input",
					"props": map[string]any{"value": "Alice"},
				},
			},
		},
	})

	result := callTool(t, srv, "query", map[string]any{
		"ids": []string{"name"},
	})

	text := resultText(t, result)
	if !strings.Contains(text, "Alice") {
		t.Errorf("query should return value 'Alice', got: %s", text)
	}
}

func TestBridgeIntegrationPatchSendsMsg(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	mp := &mockProgram{}
	bridge := NewBridge(srv, mp)

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "txt", "type": "text", "props": map[string]any{"text": "hi"}},
			},
		},
	})
	bridge.NotifyDOMChanged(imcp.MutationKindResetFocus)

	if len(mp.msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(mp.msgs))
	}
	msg, ok := mp.msgs[0].(DOMChangedMsg)
	if !ok {
		t.Errorf("expected DOMChangedMsg, got %T", mp.msgs[0])
	}
	if msg.MutationKind != imcp.MutationKindResetFocus {
		t.Errorf("MutationKind = %q, want %q", msg.MutationKind, imcp.MutationKindResetFocus)
	}
}

func TestEventContextCollectsSiblingValues(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{
					"id":   "form",
					"type": "container",
					"children": []map[string]any{
						{"id": "name_input", "type": "input", "props": map[string]any{"value": "Alice"}},
						{"id": "email_input", "type": "input", "props": map[string]any{"value": "alice@example.com"}},
						{"id": "submit_btn", "type": "button", "props": map[string]any{"label": "Submit"}},
					},
				},
			},
		},
	})

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24
	m.syncState()

	ctx := m.collectContext("submit_btn")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx["name_input"] == nil {
		t.Fatal("expected context for name_input")
	}
	if ctx["name_input"]["value"] != "Alice" {
		t.Errorf("expected name_input value 'Alice', got %v", ctx["name_input"]["value"])
	}
	if ctx["email_input"] == nil {
		t.Fatal("expected context for email_input")
	}
	if ctx["email_input"]["value"] != "alice@example.com" {
		t.Errorf("expected email value, got %v", ctx["email_input"]["value"])
	}
}

func TestFocusAdjustsAfterNodeRemoval(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "btn1", "type": "button", "props": map[string]any{"label": "A"}},
				{"id": "btn2", "type": "button", "props": map[string]any{"label": "B"}},
				{"id": "btn3", "type": "button", "props": map[string]any{"label": "C"}},
			},
		},
	})

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24
	m.syncState()
	m.focusedID = "btn2"

	callTool(t, srv, "patch", map[string]any{
		"ops": []map[string]any{
			{"op": "remove", "id": "btn2"},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	if model.focusedID == "btn2" {
		t.Error("focus should not remain on removed node")
	}
	if model.focusedID == "" {
		t.Error("focus should move to another node")
	}
}

func TestMCPCallToolDirect(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	result := callTool(t, srv, "query", map[string]any{
		"ids": []string{"root"},
	})

	text := resultText(t, result)
	if !strings.Contains(text, "root") {
		t.Errorf("query for root should mention root, got: %s", text)
	}
}

func TestMCPCallToolUnknownReturnsError(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	_, err = srv.CallTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}
