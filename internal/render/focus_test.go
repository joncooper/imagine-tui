package render

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// mustInsert inserts a node into the tree, failing the test on error.
func mustInsert(t *testing.T, tree *dom.Tree, parentID string, node *dom.Node) {
	t.Helper()
	if err := tree.Insert(parentID, node, ""); err != nil {
		t.Fatal(err)
	}
}

func makeTestTree(t *testing.T) *dom.Tree {
	t.Helper()
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}

	// Build: root > header(text) + form(name_input, email_input, submit_btn) + footer(text)
	header, _ := dom.NewNode("header", dom.TypeContainer)
	mustInsert(t, tree, "root", header)
	headerText, _ := dom.NewNode("header_text", dom.TypeText)
	mustInsert(t, tree, "header", headerText)

	form, _ := dom.NewNode("form", dom.TypeContainer)
	mustInsert(t, tree, "root", form)
	nameInput, _ := dom.NewNode("name_input", dom.TypeInput)
	mustInsert(t, tree, "form", nameInput)
	emailInput, _ := dom.NewNode("email_input", dom.TypeInput)
	mustInsert(t, tree, "form", emailInput)
	submitBtn, _ := dom.NewNode("submit_btn", dom.TypeButton)
	mustInsert(t, tree, "form", submitBtn)

	footer, _ := dom.NewNode("footer", dom.TypeContainer)
	mustInsert(t, tree, "root", footer)
	footerText, _ := dom.NewNode("footer_text", dom.TypeText)
	mustInsert(t, tree, "footer", footerText)

	return tree
}

func TestBuildFocusRing(t *testing.T) {
	tree := makeTestTree(t)
	ring := BuildFocusRing(tree)

	// Only focusable types should be in the ring.
	// In our test tree: name_input, email_input, submit_btn.
	want := []string{"name_input", "email_input", "submit_btn"}
	if len(ring.IDs) != len(want) {
		t.Fatalf("got %d focusable nodes %v, want %d %v", len(ring.IDs), ring.IDs, len(want), want)
	}
	for i, id := range want {
		if ring.IDs[i] != id {
			t.Errorf("ring[%d] = %q, want %q", i, ring.IDs[i], id)
		}
	}
}

func TestBuildFocusRingEmpty(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)
	// Add only non-focusable nodes.
	text, _ := dom.NewNode("txt", dom.TypeText)
	mustInsert(t, tree, "root", text)

	ring := BuildFocusRing(tree)
	if len(ring.IDs) != 0 {
		t.Fatalf("expected empty ring, got %v", ring.IDs)
	}
}

func TestBuildFocusRingAllTypes(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	// Add one of each focusable type.
	for i, typ := range []dom.NodeType{
		dom.TypeInput, dom.TypeTextarea, dom.TypeSelect,
		dom.TypeButton, dom.TypeTable, dom.TypeList, dom.TypeDiff,
		dom.TypeLog, dom.TypeCode,
	} {
		id := "n" + string(rune('0'+i))
		n, _ := dom.NewNode(id, typ)
		mustInsert(t, tree, "root", n)
	}

	ring := BuildFocusRing(tree)
	if len(ring.IDs) != 9 {
		t.Fatalf("expected 9 focusable nodes, got %d: %v", len(ring.IDs), ring.IDs)
	}
}

func TestBuildFocusRingIncludesOverflowScrollContainer(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	panel, _ := dom.NewNode("panel", dom.TypeContainer)
	panel.SetProp("overflow", "scroll")
	panel.SetProp("height", 5)
	mustInsert(t, tree, "root", panel)

	body, _ := dom.NewNode("body", dom.TypeText)
	mustInsert(t, tree, "panel", body)

	ring := BuildFocusRing(tree)
	if !ring.Contains("panel") {
		t.Fatalf("expected overflow-scrolling container to be focusable, got %v", ring.IDs)
	}
}

func TestFocusRingFocusablePropOverride(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	// A text node is normally not focusable.
	text, _ := dom.NewNode("focusable_text", dom.TypeText)
	text.SetProp("focusable", true)
	mustInsert(t, tree, "root", text)

	// An input node is normally focusable, but can opt out.
	input, _ := dom.NewNode("nonfocusable_input", dom.TypeInput)
	input.SetProp("focusable", false)
	mustInsert(t, tree, "root", input)

	ring := BuildFocusRing(tree)
	if len(ring.IDs) != 1 {
		t.Fatalf("expected 1 focusable node, got %d: %v", len(ring.IDs), ring.IDs)
	}
	if ring.IDs[0] != "focusable_text" {
		t.Errorf("got %q, want focusable_text", ring.IDs[0])
	}
}

func TestFocusRingNext(t *testing.T) {
	tree := makeTestTree(t)
	ring := BuildFocusRing(tree)

	tests := []struct {
		current string
		want    string
	}{
		{"name_input", "email_input"},
		{"email_input", "submit_btn"},
		{"submit_btn", "name_input"}, // wraps around
		{"", "name_input"},           // no current focus → first
		{"nonexistent", "name_input"},
	}
	for _, tt := range tests {
		got := ring.Next(tt.current)
		if got != tt.want {
			t.Errorf("Next(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestFocusRingPrev(t *testing.T) {
	tree := makeTestTree(t)
	ring := BuildFocusRing(tree)

	tests := []struct {
		current string
		want    string
	}{
		{"submit_btn", "email_input"},
		{"email_input", "name_input"},
		{"name_input", "submit_btn"}, // wraps around
		{"", "submit_btn"},           // no current → last
		{"nonexistent", "submit_btn"},
	}
	for _, tt := range tests {
		got := ring.Prev(tt.current)
		if got != tt.want {
			t.Errorf("Prev(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestFocusRingNextPrevEmpty(t *testing.T) {
	ring := &FocusRing{}
	if got := ring.Next("anything"); got != "" {
		t.Errorf("Next on empty ring = %q, want empty", got)
	}
	if got := ring.Prev("anything"); got != "" {
		t.Errorf("Prev on empty ring = %q, want empty", got)
	}
}

func TestFocusRingContains(t *testing.T) {
	tree := makeTestTree(t)
	ring := BuildFocusRing(tree)

	if !ring.Contains("name_input") {
		t.Error("expected ring to contain name_input")
	}
	if ring.Contains("header_text") {
		t.Error("expected ring to not contain header_text")
	}
	if ring.Contains("nonexistent") {
		t.Error("expected ring to not contain nonexistent")
	}
}

func TestFocusRingAfterNodeRemoval(t *testing.T) {
	tree := makeTestTree(t)

	// Remove email_input from tree.
	if _, err := tree.Remove("email_input"); err != nil {
		t.Fatal(err)
	}

	ring := BuildFocusRing(tree)
	want := []string{"name_input", "submit_btn"}
	if len(ring.IDs) != len(want) {
		t.Fatalf("got %d, want %d: %v", len(ring.IDs), len(want), ring.IDs)
	}
	for i, id := range want {
		if ring.IDs[i] != id {
			t.Errorf("ring[%d] = %q, want %q", i, ring.IDs[i], id)
		}
	}
}

func TestFocusRingContainerTrapping(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	// A modal container that traps focus.
	modal, _ := dom.NewNode("modal", dom.TypeContainer)
	modal.SetProp("focus_trap", true)
	mustInsert(t, tree, "root", modal)

	// Outside the modal.
	outsideBtn, _ := dom.NewNode("outside_btn", dom.TypeButton)
	mustInsert(t, tree, "root", outsideBtn)

	// Inside the modal.
	modalInput, _ := dom.NewNode("modal_input", dom.TypeInput)
	mustInsert(t, tree, "modal", modalInput)
	modalBtn, _ := dom.NewNode("modal_btn", dom.TypeButton)
	mustInsert(t, tree, "modal", modalBtn)

	// When focus is inside the trapped container, Next/Prev should only cycle within it.
	ring := BuildFocusRing(tree)
	trapped := ring.NextInTrap("modal_input", tree)
	if trapped != "modal_btn" {
		t.Errorf("NextInTrap from modal_input = %q, want modal_btn", trapped)
	}
	trapped = ring.NextInTrap("modal_btn", tree)
	if trapped != "modal_input" {
		t.Errorf("NextInTrap from modal_btn = %q, want modal_input (wrap within trap)", trapped)
	}

	// When focus is outside the trap, normal cycling includes everything.
	next := ring.Next("outside_btn")
	// outside_btn is after the modal children in DFS order.
	// DFS: modal_input, modal_btn, outside_btn → next wraps to modal_input.
	if next != "modal_input" {
		t.Errorf("Next from outside_btn = %q, want modal_input", next)
	}
}

func TestFocusRingPrevInTrap(t *testing.T) {
	root, _ := dom.NewNode("root", dom.TypeContainer)
	tree, _ := dom.NewTree(root)

	modal, _ := dom.NewNode("modal", dom.TypeContainer)
	modal.SetProp("focus_trap", true)
	mustInsert(t, tree, "root", modal)

	outsideBtn, _ := dom.NewNode("outside_btn", dom.TypeButton)
	mustInsert(t, tree, "root", outsideBtn)

	modalInput, _ := dom.NewNode("modal_input", dom.TypeInput)
	mustInsert(t, tree, "modal", modalInput)
	modalBtn, _ := dom.NewNode("modal_btn", dom.TypeButton)
	mustInsert(t, tree, "modal", modalBtn)

	ring := BuildFocusRing(tree)
	trapped := ring.PrevInTrap("modal_input", tree)
	if trapped != "modal_btn" {
		t.Errorf("PrevInTrap from modal_input = %q, want modal_btn (wrap within trap)", trapped)
	}
	trapped = ring.PrevInTrap("modal_btn", tree)
	if trapped != "modal_input" {
		t.Errorf("PrevInTrap from modal_btn = %q, want modal_input", trapped)
	}
}
