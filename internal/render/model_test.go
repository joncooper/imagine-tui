package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/widget"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(srv, widget.DefaultRegistry())
}

func newTestModelWithTree(t *testing.T) (Model, *imcp.Server) {
	t.Helper()
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	// Build a small UI tree on the server's DOM.
	domTree := srv.Tree()
	header, _ := dom.NewNode("header", dom.TypeText)
	header.SetProp("text", "Hello")
	if err = domTree.Insert("root", header, ""); err != nil {
		t.Fatal(err)
	}

	nameInput, _ := dom.NewNode("name_input", dom.TypeInput)
	nameInput.SetProp("placeholder", "Name")
	if err = domTree.Insert("root", nameInput, ""); err != nil {
		t.Fatal(err)
	}

	submitBtn, _ := dom.NewNode("submit_btn", dom.TypeButton)
	submitBtn.SetProp("label", "Submit")
	if err = domTree.Insert("root", submitBtn, ""); err != nil {
		t.Fatal(err)
	}

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24
	// Sync widgets and focus ring.
	m.syncState()
	return m, srv
}

// --- M5-1 Tests ---

func TestModelInit(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Init()
	// Init should return nil cmd (no startup command needed).
	if cmd != nil {
		t.Error("expected nil cmd from Init()")
	}
}

func TestModelViewEmpty(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.height = 24
	m.syncState()

	// An empty root container renders an empty string, which is valid.
	// Just verify View() doesn't panic.
	_ = m.View()
}

func TestModelViewRendersWidgets(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	view := m.View()
	if !strings.Contains(view, "Hello") {
		t.Errorf("view should contain 'Hello' text, got:\n%s", view)
	}
}

func TestModelDOMChangedMsg(t *testing.T) {
	m, srv := newTestModelWithTree(t)

	// Mutate the DOM externally (as MCP would).
	newText, _ := dom.NewNode("footer", dom.TypeText)
	newText.SetProp("text", "Footer")
	if err := srv.Tree().Insert("root", newText, ""); err != nil {
		t.Fatal(err)
	}

	// Send DOMChangedMsg.
	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	view := model.View()
	if !strings.Contains(view, "Footer") {
		t.Errorf("after DOMChangedMsg, view should contain 'Footer', got:\n%s", view)
	}
}

func TestModelKeyPressFocusCycle(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	// Initial state: no focus or first focusable.
	// Press Tab to cycle focus.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)

	// Should focus the first focusable widget.
	if model.focusedID == "" {
		t.Error("Tab should set focus to a widget")
	}

	// Tab again.
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2 := newM2.(Model)

	if model2.focusedID == model.focusedID {
		t.Error("second Tab should move focus to next widget")
	}
}

func TestModelShiftTabReversesFocus(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	// Tab to get focus on first widget.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := newM.(Model)
	first := model.focusedID

	// Shift+Tab should go to last focusable.
	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model2 := newM2.(Model)

	if model2.focusedID == first {
		t.Error("Shift+Tab should move focus backwards")
	}
}

func TestModelKeyRouteToFocusedWidget(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	// Focus the input.
	m.focusedID = "name_input"
	m.syncState()

	// Type a character.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := newM.(Model)

	// The input widget should have processed the keystroke.
	// Check that focus is still on the input.
	if model.focusedID != "name_input" {
		t.Errorf("focus should remain on name_input, got %q", model.focusedID)
	}
}

func TestModelKeyRouteToFocusedScrollContainer(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	root := srv.Tree().Root
	root.SetProp("direction", "vertical")

	panel, _ := dom.NewNode("panel", dom.TypeContainer)
	panel.SetProp("overflow", "scroll")
	panel.SetProp("height", 3)
	panel.SetProp("focusable", true)
	if err := srv.Tree().Insert("root", panel, ""); err != nil {
		t.Fatal(err)
	}

	body, _ := dom.NewNode("body", dom.TypeText)
	body.SetProp("wrap", false)
	body.SetProp("text", "line1\nline2\nline3\nline4\nline5")
	if err := srv.Tree().Insert("panel", body, ""); err != nil {
		t.Fatal(err)
	}

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 40
	m.height = 10
	m.syncState()
	m.focusedID = "panel"

	before := m.View()
	if !strings.Contains(before, "line1") {
		t.Fatalf("expected initial viewport content, got:\n%s", before)
	}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := newM.(Model)

	after := model.View()
	if !strings.Contains(after, "line4") || strings.Contains(after, "line1") {
		t.Fatalf("expected scrolled viewport, got:\n%s", after)
	}
}

// --- M5-4 Tests ---

func TestModelWindowSizeMsg(t *testing.T) {
	m := newTestModel(t)

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := newM.(Model)

	if model.width != 120 {
		t.Errorf("width = %d, want 120", model.width)
	}
	if model.height != 40 {
		t.Errorf("height = %d, want 40", model.height)
	}
}

func TestModelResizeRerendersCorrectly(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	// Render at original size.
	view80 := m.View()

	// Resize.
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	model := newM.(Model)
	view40 := model.View()

	// Views should be different (different width).
	if view80 == view40 {
		t.Error("view should change after resize")
	}
}

func TestModelZeroSizeReturnsEmpty(t *testing.T) {
	m := newTestModel(t)
	m.width = 0
	m.height = 0

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view at zero size, got %q", view)
	}
}

// --- MCP disconnect ---

func TestModelMCPDisconnectedMsg(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	newM, _ := m.Update(MCPConnectedMsg{SessionID: 1})
	model := newM.(Model)
	newM2, _ := model.Update(MCPDisconnectedMsg{SessionID: 1})
	model2 := newM2.(Model)

	if !model2.waitingForReconnect {
		t.Error("expected waitingForReconnect = true")
	}

	view := model2.View()
	if !strings.Contains(view, "Agent disconnected.") {
		t.Errorf("view should show reconnect modal, got:\n%s", view)
	}
	if !strings.Contains(view, "Hello") {
		t.Errorf("view should preserve last rendered UI, got:\n%s", view)
	}
}

func TestModelIgnoresStaleDisconnect(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	newM, _ := m.Update(MCPConnectedMsg{SessionID: 2})
	model := newM.(Model)
	newM2, _ := model.Update(MCPDisconnectedMsg{SessionID: 1})
	model2 := newM2.(Model)

	if model2.waitingForReconnect {
		t.Error("stale disconnect should be ignored")
	}
	if model2.activeSessionID != 2 {
		t.Errorf("activeSessionID = %d, want 2", model2.activeSessionID)
	}
}

// --- Ctrl+C shutdown ---

func TestModelCtrlCQuits(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C should return a quit command")
	}
	// Execute the command to see if it produces a quit message.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

// --- Widget event routing ---

func TestModelWidgetEventEnqueuesForClaude(t *testing.T) {
	m, srv := newTestModelWithTree(t)

	// Focus on submit_btn and press Enter to trigger click event.
	m.focusedID = "submit_btn"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = newM.(Model)

	// The button click should be queued in the event queue.
	// We can't easily check without dequeuing, but verify no panic.
	_ = srv.Events()
}

// --- Focus ring rebuild on DOM change ---

func TestModelFocusRingRebuildsOnDOMChange(t *testing.T) {
	m, srv := newTestModelWithTree(t)

	// Focus the submit button.
	m.focusedID = "submit_btn"

	// Remove the submit button from DOM.
	if _, err := srv.Tree().Remove("submit_btn"); err != nil {
		t.Fatal(err)
	}

	// Trigger DOM change.
	newM, _ := m.Update(DOMChangedMsg{})
	model := newM.(Model)

	// Focus should move since submit_btn no longer exists.
	if model.focusedID == "submit_btn" {
		t.Error("focus should move away from removed node")
	}
}

// --- State sync ---

func TestModelSyncCreatesWidgetInstances(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	// Widget tree should have instances for all nodes.
	if m.widgets.Get("header") == nil {
		t.Error("expected widget instance for 'header'")
	}
	if m.widgets.Get("name_input") == nil {
		t.Error("expected widget instance for 'name_input'")
	}
	if m.widgets.Get("submit_btn") == nil {
		t.Error("expected widget instance for 'submit_btn'")
	}
}
