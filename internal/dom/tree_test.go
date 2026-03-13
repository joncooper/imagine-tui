package dom

import (
	"testing"
)

// helper to build a test tree:
//
//	root
//	├── a
//	│   ├── a1
//	│   └── a2
//	└── b
//	    └── b1
func makeTestTree(t *testing.T) *Tree {
	t.Helper()
	root, _ := NewNode("root", TypeContainer)
	a, _ := NewNode("a", TypeContainer)
	a1, _ := NewNode("a1", TypeText)
	a2, _ := NewNode("a2", TypeText)
	b, _ := NewNode("b", TypeContainer)
	b1, _ := NewNode("b1", TypeButton)

	a.Children = append(a.Children, a1, a2)
	a1.parent = a
	a2.parent = a
	b.Children = append(b.Children, b1)
	b1.parent = b
	root.Children = append(root.Children, a, b)
	a.parent = root
	b.parent = root

	tree, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestNewTree(t *testing.T) {
	t.Run("valid tree", func(t *testing.T) {
		root, _ := NewNode("root", TypeContainer)
		tree, err := NewTree(root)
		if err != nil {
			t.Fatal(err)
		}
		if tree.Root != root {
			t.Error("root mismatch")
		}
	})

	t.Run("nil root", func(t *testing.T) {
		_, err := NewTree(nil)
		if err == nil {
			t.Fatal("expected error for nil root")
		}
	})

	t.Run("duplicate IDs", func(t *testing.T) {
		root, _ := NewNode("dup", TypeContainer)
		child, _ := NewNode("dup", TypeText)
		child.parent = root
		root.Children = append(root.Children, child)
		_, err := NewTree(root)
		if err == nil {
			t.Fatal("expected error for duplicate IDs")
		}
	})
}

func TestTreeFind(t *testing.T) {
	tree := makeTestTree(t)

	tests := []struct {
		id    string
		found bool
	}{
		{"root", true},
		{"a", true},
		{"a1", true},
		{"a2", true},
		{"b", true},
		{"b1", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			n := tree.Find(tt.id)
			if tt.found && n == nil {
				t.Error("expected to find node")
			}
			if !tt.found && n != nil {
				t.Error("expected nil")
			}
			if tt.found && n != nil && n.ID != tt.id {
				t.Errorf("found wrong node: %q", n.ID)
			}
		})
	}
}

func TestTreeInsert(t *testing.T) {
	tests := []struct {
		name     string
		parentID string
		nodeID   string
		afterID  string
		wantErr  bool
		errMsg   string
	}{
		{name: "append to root", parentID: "root", nodeID: "c"},
		{name: "append to leaf", parentID: "a1", nodeID: "a1child"},
		{name: "after sibling", parentID: "a", nodeID: "a1.5", afterID: "a1"},
		{name: "parent not found", parentID: "nope", nodeID: "x", wantErr: true, errMsg: "not found"},
		{name: "sibling not found", parentID: "a", nodeID: "x", afterID: "nope", wantErr: true, errMsg: "sibling"},
		{name: "duplicate ID", parentID: "root", nodeID: "a", wantErr: true, errMsg: "duplicate"},
		{name: "nil node", parentID: "root", nodeID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := makeTestTree(t)
			var node *Node
			if tt.nodeID != "" {
				node, _ = NewNode(tt.nodeID, TypeText)
			}
			err := tree.Insert(tt.parentID, node, tt.afterID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// Verify the node is findable.
			found := tree.Find(tt.nodeID)
			if found == nil {
				t.Fatal("inserted node not found")
			}
			if found.Parent() == nil {
				t.Fatal("inserted node has nil parent")
			}
		})
	}

	t.Run("insert after positions correctly", func(t *testing.T) {
		tree := makeTestTree(t)
		mid, _ := NewNode("mid", TypeText)
		if err := tree.Insert("a", mid, "a1"); err != nil {
			t.Fatal(err)
		}
		a := tree.Find("a")
		if len(a.Children) != 3 {
			t.Fatalf("expected 3 children, got %d", len(a.Children))
		}
		if a.Children[0].ID != "a1" || a.Children[1].ID != "mid" || a.Children[2].ID != "a2" {
			t.Errorf("children order: %s, %s, %s", a.Children[0].ID, a.Children[1].ID, a.Children[2].ID)
		}
	})

	t.Run("insert subtree with children", func(t *testing.T) {
		tree := makeTestTree(t)
		sub, _ := NewNode("sub", TypeContainer)
		subChild, _ := NewNode("subchild", TypeText)
		subChild.parent = sub
		sub.Children = append(sub.Children, subChild)

		if err := tree.Insert("root", sub, ""); err != nil {
			t.Fatal(err)
		}
		if tree.Find("sub") == nil {
			t.Error("subtree root not indexed")
		}
		if tree.Find("subchild") == nil {
			t.Error("subtree child not indexed")
		}
	})
}

func TestTreeRemove(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
		errMsg  string
	}{
		{name: "remove leaf", id: "b1"},
		{name: "remove subtree", id: "a"},
		{name: "remove root fails", id: "root", wantErr: true, errMsg: "cannot remove root"},
		{name: "remove nonexistent", id: "nope", wantErr: true, errMsg: "not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := makeTestTree(t)
			removed, err := tree.Remove(tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if removed == nil {
				t.Fatal("removed node is nil")
			}
			if removed.ID != tt.id {
				t.Errorf("removed ID %q, want %q", removed.ID, tt.id)
			}
			// Node should no longer be in the index.
			if tree.Find(tt.id) != nil {
				t.Error("removed node still in index")
			}
		})
	}

	t.Run("remove subtree clears all descendants from index", func(t *testing.T) {
		tree := makeTestTree(t)
		_, err := tree.Remove("a")
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"a", "a1", "a2"} {
			if tree.Find(id) != nil {
				t.Errorf("descendant %q still in index after subtree removal", id)
			}
		}
	})
}

func TestTreeMove(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		newParentID string
		afterID     string
		wantErr     bool
		errMsg      string
	}{
		{name: "move leaf", id: "b1", newParentID: "a"},
		{name: "move with afterID", id: "b1", newParentID: "a", afterID: "a1"},
		{name: "move node not found", id: "nope", newParentID: "root", wantErr: true, errMsg: "not found"},
		{name: "move root", id: "root", newParentID: "a", wantErr: true, errMsg: "cannot move root"},
		{name: "move to nonexistent parent", id: "b1", newParentID: "nope", wantErr: true, errMsg: "not found"},
		{name: "move to descendant (cycle)", id: "a", newParentID: "a1", wantErr: true, errMsg: "descendant"},
		{name: "move to self (cycle)", id: "a", newParentID: "a", wantErr: true, errMsg: "descendant"},
		{name: "move with bad afterID", id: "b1", newParentID: "a", afterID: "nope", wantErr: true, errMsg: "sibling"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := makeTestTree(t)
			err := tree.Move(tt.id, tt.newParentID, tt.afterID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			node := tree.Find(tt.id)
			if node == nil {
				t.Fatal("moved node not found")
			}
			if node.Parent().ID != tt.newParentID {
				t.Errorf("parent = %q, want %q", node.Parent().ID, tt.newParentID)
			}
		})
	}

	t.Run("move reparents correctly", func(t *testing.T) {
		tree := makeTestTree(t)
		// Move b1 from b to a, after a1.
		if err := tree.Move("b1", "a", "a1"); err != nil {
			t.Fatal(err)
		}
		a := tree.Find("a")
		if len(a.Children) != 3 {
			t.Fatalf("expected 3 children of a, got %d", len(a.Children))
		}
		if a.Children[1].ID != "b1" {
			t.Errorf("b1 not in correct position: %s", a.Children[1].ID)
		}
		b := tree.Find("b")
		if len(b.Children) != 0 {
			t.Errorf("b should have 0 children, got %d", len(b.Children))
		}
	})
}

func TestTreeWalk(t *testing.T) {
	tree := makeTestTree(t)
	var ids []string
	tree.Walk(func(n *Node) bool {
		ids = append(ids, n.ID)
		return true
	})
	expected := []string{"root", "a", "a1", "a2", "b", "b1"}
	if len(ids) != len(expected) {
		t.Fatalf("walk visited %d nodes, want %d", len(ids), len(expected))
	}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("walk[%d] = %q, want %q", i, ids[i], id)
		}
	}
}

func TestTreeWalkEarlyStop(t *testing.T) {
	tree := makeTestTree(t)
	var ids []string
	tree.Walk(func(n *Node) bool {
		ids = append(ids, n.ID)
		return n.ID != "a" // stop after processing "a"'s entry
	})
	// Should visit root, a, then stop (a returns false so children aren't visited).
	if len(ids) != 2 {
		t.Fatalf("walk visited %d nodes, want 2: %v", len(ids), ids)
	}
}

func TestTreeSummary(t *testing.T) {
	tree := makeTestTree(t)
	got := tree.Summary()
	expected := "root(a(a1, a2), b(b1))"
	if got != expected {
		t.Errorf("Summary = %q, want %q", got, expected)
	}
}

func TestTreeSummarySingleNode(t *testing.T) {
	root, _ := NewNode("root", TypeContainer)
	tree, _ := NewTree(root)
	if tree.Summary() != "root" {
		t.Errorf("Summary = %q, want root", tree.Summary())
	}
}

func TestTreeSummaryNilRoot(t *testing.T) {
	tree := &Tree{Root: nil}
	if tree.Summary() != "" {
		t.Errorf("Summary of nil root = %q, want empty", tree.Summary())
	}
}

func TestRebuildFullIndex(t *testing.T) {
	tree := makeTestTree(t)

	// Manually corrupt the index.
	tree.index = make(map[string]*Node)

	// Rebuild should restore it.
	if err := tree.RebuildFullIndex(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"root", "a", "a1", "a2", "b", "b1"} {
		if tree.Find(id) == nil {
			t.Errorf("%q not found after RebuildFullIndex", id)
		}
	}
}

func TestTreeInsertAtEnd(t *testing.T) {
	tree := makeTestTree(t)
	last, _ := NewNode("last", TypeText)
	if err := tree.Insert("a", last, "a2"); err != nil {
		t.Fatal(err)
	}
	a := tree.Find("a")
	if a.Children[len(a.Children)-1].ID != "last" {
		t.Error("expected 'last' at end of children")
	}
}

func TestTreeRemoveUpdatesParentChildCount(t *testing.T) {
	tree := makeTestTree(t)
	b := tree.Find("b")
	if len(b.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(b.Children))
	}
	if _, err := tree.Remove("b1"); err != nil {
		t.Fatal(err)
	}
	if len(b.Children) != 0 {
		t.Errorf("expected 0 children after remove, got %d", len(b.Children))
	}
}

func TestTreeMoveSameParent(t *testing.T) {
	tree := makeTestTree(t)
	if err := tree.Move("a2", "a", ""); err != nil {
		t.Fatal(err)
	}
	a := tree.Find("a")
	if a.Children[len(a.Children)-1].ID != "a2" {
		t.Errorf("a2 should be last child, got %s", a.Children[len(a.Children)-1].ID)
	}
}

func TestTreeWalkSingleNode(t *testing.T) {
	root, _ := NewNode("solo", TypeContainer)
	tree, _ := NewTree(root)
	var ids []string
	tree.Walk(func(n *Node) bool {
		ids = append(ids, n.ID)
		return true
	})
	if len(ids) != 1 || ids[0] != "solo" {
		t.Errorf("walk on single node: %v", ids)
	}
}

// --- Template-driven tree methods ---

// makeTemplatedTree builds a tree with a container that has an item_template prop.
func makeTemplatedTree(t *testing.T) *Tree {
	t.Helper()
	root, _ := NewNode("root", TypeContainer)
	list, _ := NewNode("log-list", TypeContainer)
	list.SetProp("item_template", map[string]any{
		"type": "container",
		"props": map[string]any{
			"direction": "row",
		},
		"children": []any{
			map[string]any{"type": "text", "props": map[string]any{"content": "{{level}}"}},
			map[string]any{"type": "text", "props": map[string]any{"content": "{{msg}}"}},
		},
	})
	root.Children = append(root.Children, list)
	list.parent = root
	tree, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestTree_SetItems_Basic(t *testing.T) {
	tree := makeTemplatedTree(t)
	items := []map[string]any{
		{"level": "INFO", "msg": "started"},
		{"level": "ERROR", "msg": "disk full"},
	}

	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}

	list := tree.Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}

	// Verify first item
	row0 := list.Children[0]
	if row0.ID != "log-list-0" {
		t.Errorf("row0.ID = %q, want %q", row0.ID, "log-list-0")
	}
	if len(row0.Children) != 2 {
		t.Fatalf("row0 children = %d, want 2", len(row0.Children))
	}
	if v, _ := row0.Children[0].GetProp("content"); v != "INFO" {
		t.Errorf("row0.child[0].content = %v, want INFO", v)
	}
	if v, _ := row0.Children[1].GetProp("content"); v != "started" {
		t.Errorf("row0.child[1].content = %v, want 'started'", v)
	}

	// Verify second item
	row1 := list.Children[1]
	if row1.ID != "log-list-1" {
		t.Errorf("row1.ID = %q, want %q", row1.ID, "log-list-1")
	}

	// Verify all nodes are indexed
	if tree.Find("log-list-0") == nil {
		t.Error("log-list-0 not in index")
	}
	if tree.Find("log-list-0-0") == nil {
		t.Error("log-list-0-0 not in index")
	}
}

func TestTree_SetItems_ReplacesExisting(t *testing.T) {
	tree := makeTemplatedTree(t)

	// First set
	if err := tree.SetItems("log-list", []map[string]any{
		{"level": "INFO", "msg": "first"},
	}); err != nil {
		t.Fatal(err)
	}

	// Second set replaces
	if err := tree.SetItems("log-list", []map[string]any{
		{"level": "ERROR", "msg": "second"},
		{"level": "WARN", "msg": "third"},
	}); err != nil {
		t.Fatal(err)
	}

	list := tree.Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}
	if v, _ := list.Children[0].Children[1].GetProp("content"); v != "second" {
		t.Errorf("content = %v, want 'second'", v)
	}

	// Old nodes should be gone from index
	// (the first set's node IDs are reused, but let's verify tree is correct)
	if tree.Find("log-list-0") == nil {
		t.Error("log-list-0 should exist from second set")
	}
}

func TestTree_SetItems_MissingTarget(t *testing.T) {
	tree := makeTemplatedTree(t)
	err := tree.SetItems("nonexistent", []map[string]any{{"k": "v"}})
	if err == nil {
		t.Fatal("expected error for missing target")
	}
}

func TestTree_SetItems_NoTemplate(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.SetItems("a", []map[string]any{{"k": "v"}})
	if err == nil {
		t.Fatal("expected error for missing item_template")
	}
}

func TestTree_SetItems_EmptyItems(t *testing.T) {
	tree := makeTemplatedTree(t)
	// First add some items
	if err := tree.SetItems("log-list", []map[string]any{
		{"level": "INFO", "msg": "x"},
	}); err != nil {
		t.Fatal(err)
	}
	// Then clear with empty items
	if err := tree.SetItems("log-list", nil); err != nil {
		t.Fatal(err)
	}
	list := tree.Find("log-list")
	if len(list.Children) != 0 {
		t.Errorf("children = %d, want 0", len(list.Children))
	}
}

func TestTree_SetItems_WithKeys(t *testing.T) {
	tree := makeTemplatedTree(t)
	items := []map[string]any{
		{"key": "err1", "level": "ERROR", "msg": "disk full"},
		{"key": "err2", "level": "WARN", "msg": "retry"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}

	if tree.Find("log-list-err1") == nil {
		t.Error("log-list-err1 not in index")
	}
	if tree.Find("log-list-err2") == nil {
		t.Error("log-list-err2 not in index")
	}
}

func TestTree_AppendItems_Basic(t *testing.T) {
	tree := makeTemplatedTree(t)

	// Append to empty container
	if err := tree.AppendItems("log-list", []map[string]any{
		{"level": "INFO", "msg": "first"},
	}); err != nil {
		t.Fatal(err)
	}

	list := tree.Find("log-list")
	if len(list.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(list.Children))
	}

	// Append more
	if err := tree.AppendItems("log-list", []map[string]any{
		{"level": "WARN", "msg": "second"},
	}); err != nil {
		t.Fatal(err)
	}

	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}

	// First item still there
	if v, _ := list.Children[0].Children[1].GetProp("content"); v != "first" {
		t.Errorf("child[0] content = %v, want 'first'", v)
	}
	// Second item appended
	if v, _ := list.Children[1].Children[1].GetProp("content"); v != "second" {
		t.Errorf("child[1] content = %v, want 'second'", v)
	}
}

func TestTree_AppendItems_ToExisting(t *testing.T) {
	tree := makeTemplatedTree(t)

	// Set initial items
	if err := tree.SetItems("log-list", []map[string]any{
		{"level": "INFO", "msg": "initial"},
	}); err != nil {
		t.Fatal(err)
	}

	// Append additional items
	if err := tree.AppendItems("log-list", []map[string]any{
		{"level": "ERROR", "msg": "appended"},
	}); err != nil {
		t.Fatal(err)
	}

	list := tree.Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}
}

func TestTree_RemoveItems_ByKey(t *testing.T) {
	tree := makeTemplatedTree(t)
	items := []map[string]any{
		{"key": "a", "level": "INFO", "msg": "keep"},
		{"key": "b", "level": "ERROR", "msg": "remove"},
		{"key": "c", "level": "WARN", "msg": "keep"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}

	if err := tree.RemoveItems("log-list", []string{"b"}); err != nil {
		t.Fatal(err)
	}

	list := tree.Find("log-list")
	if len(list.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(list.Children))
	}
	if tree.Find("log-list-b") != nil {
		t.Error("log-list-b should have been removed")
	}
	if tree.Find("log-list-a") == nil {
		t.Error("log-list-a should still exist")
	}
	if tree.Find("log-list-c") == nil {
		t.Error("log-list-c should still exist")
	}
}

func TestTree_RemoveItems_Nonexistent(t *testing.T) {
	tree := makeTemplatedTree(t)
	if err := tree.SetItems("log-list", []map[string]any{
		{"key": "a", "level": "INFO", "msg": "x"},
	}); err != nil {
		t.Fatal(err)
	}

	err := tree.RemoveItems("log-list", []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestTree_RemoveItems_MissingTarget(t *testing.T) {
	tree := makeTemplatedTree(t)
	err := tree.RemoveItems("nonexistent", []string{"a"})
	if err == nil {
		t.Fatal("expected error for missing target")
	}
}

// makeListTree builds a tree with a list node for testing list-type set_items.
func makeListTree(t *testing.T) *Tree {
	t.Helper()
	root, _ := NewNode("root", TypeContainer)
	list, _ := NewNode("log-list", TypeList)
	root.Children = append(root.Children, list)
	list.parent = root

	tree, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestTree_SetItems_List_Basic(t *testing.T) {
	tree := makeListTree(t)
	items := []map[string]any{
		{"id": "1", "label": "INFO: server started", "badge": "INFO"},
		{"id": "2", "label": "ERROR: disk full", "badge": "ERROR"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("log-list")
	rawItems, ok := node.GetProp("items")
	if !ok {
		t.Fatal("items prop not set")
	}
	// Items should be stored as []any for PropMapSlice compatibility.
	anySlice, ok := rawItems.([]any)
	if !ok {
		t.Fatalf("items prop is %T, want []any", rawItems)
	}
	if len(anySlice) != 2 {
		t.Fatalf("got %d items, want 2", len(anySlice))
	}
	first := anySlice[0].(map[string]any)
	if first["label"] != "INFO: server started" {
		t.Errorf("first label = %q, want %q", first["label"], "INFO: server started")
	}
}

func TestTree_SetItems_List_ReplacesExisting(t *testing.T) {
	tree := makeListTree(t)
	items1 := []map[string]any{
		{"id": "1", "label": "old item"},
	}
	if err := tree.SetItems("log-list", items1); err != nil {
		t.Fatal(err)
	}
	items2 := []map[string]any{
		{"id": "a", "label": "new item A"},
		{"id": "b", "label": "new item B"},
	}
	if err := tree.SetItems("log-list", items2); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("log-list")
	anySlice := node.Props["items"].([]any)
	if len(anySlice) != 2 {
		t.Fatalf("got %d items, want 2", len(anySlice))
	}
	if anySlice[0].(map[string]any)["label"] != "new item A" {
		t.Error("first item not replaced")
	}
}

func TestTree_SetItems_List_EmptyClears(t *testing.T) {
	tree := makeListTree(t)
	items := []map[string]any{
		{"id": "1", "label": "item"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}
	if err := tree.SetItems("log-list", nil); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("log-list")
	anySlice := node.Props["items"].([]any)
	if len(anySlice) != 0 {
		t.Fatalf("got %d items, want 0", len(anySlice))
	}
}

func TestTree_AppendItems_List_Basic(t *testing.T) {
	tree := makeListTree(t)
	items := []map[string]any{
		{"id": "1", "label": "first"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}
	more := []map[string]any{
		{"id": "2", "label": "second"},
		{"id": "3", "label": "third"},
	}
	if err := tree.AppendItems("log-list", more); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("log-list")
	anySlice := node.Props["items"].([]any)
	if len(anySlice) != 3 {
		t.Fatalf("got %d items, want 3", len(anySlice))
	}
	if anySlice[2].(map[string]any)["label"] != "third" {
		t.Error("third item not appended")
	}
}

func TestTree_AppendItems_List_ToEmpty(t *testing.T) {
	tree := makeListTree(t)
	items := []map[string]any{
		{"id": "1", "label": "first"},
	}
	if err := tree.AppendItems("log-list", items); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("log-list")
	anySlice := node.Props["items"].([]any)
	if len(anySlice) != 1 {
		t.Fatalf("got %d items, want 1", len(anySlice))
	}
}

func TestTree_RemoveItems_List_ByID(t *testing.T) {
	tree := makeListTree(t)
	items := []map[string]any{
		{"id": "a", "label": "alpha"},
		{"id": "b", "label": "beta"},
		{"id": "c", "label": "charlie"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}
	if err := tree.RemoveItems("log-list", []string{"b"}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("log-list")
	anySlice := node.Props["items"].([]any)
	if len(anySlice) != 2 {
		t.Fatalf("got %d items, want 2", len(anySlice))
	}
	// Remaining should be a and c.
	if anySlice[0].(map[string]any)["id"] != "a" {
		t.Error("expected first item to be 'a'")
	}
	if anySlice[1].(map[string]any)["id"] != "c" {
		t.Error("expected second item to be 'c'")
	}
}

func TestTree_RemoveItems_List_Nonexistent(t *testing.T) {
	tree := makeListTree(t)
	items := []map[string]any{
		{"id": "a", "label": "alpha"},
	}
	if err := tree.SetItems("log-list", items); err != nil {
		t.Fatal(err)
	}
	err := tree.RemoveItems("log-list", []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

// --- Table-type set_items tests ---

func makeTableTree(t *testing.T) *Tree {
	t.Helper()
	root, _ := NewNode("root", TypeContainer)
	tbl, _ := NewNode("data-table", TypeTable)
	root.Children = append(root.Children, tbl)
	tbl.parent = root

	tree, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestTree_SetItems_Table_Basic(t *testing.T) {
	tree := makeTableTree(t)
	rows := []map[string]any{
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25},
	}
	if err := tree.SetItems("data-table", rows); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("data-table")
	rawRows, ok := node.GetProp("rows")
	if !ok {
		t.Fatal("rows prop not set")
	}
	anySlice, ok := rawRows.([]any)
	if !ok {
		t.Fatalf("rows prop is %T, want []any", rawRows)
	}
	if len(anySlice) != 2 {
		t.Fatalf("got %d rows, want 2", len(anySlice))
	}
	first := anySlice[0].(map[string]any)
	if first["name"] != "Alice" {
		t.Errorf("first name = %v, want Alice", first["name"])
	}
}

func TestTree_AppendItems_Table_Basic(t *testing.T) {
	tree := makeTableTree(t)
	if err := tree.SetItems("data-table", []map[string]any{
		{"name": "Alice"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tree.AppendItems("data-table", []map[string]any{
		{"name": "Bob"},
	}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("data-table")
	anySlice := node.Props["rows"].([]any)
	if len(anySlice) != 2 {
		t.Fatalf("got %d rows, want 2", len(anySlice))
	}
}

func TestTree_RemoveItems_Table_ByID(t *testing.T) {
	tree := makeTableTree(t)
	if err := tree.SetItems("data-table", []map[string]any{
		{"id": "a", "name": "Alice"},
		{"id": "b", "name": "Bob"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tree.RemoveItems("data-table", []string{"a"}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("data-table")
	anySlice := node.Props["rows"].([]any)
	if len(anySlice) != 1 {
		t.Fatalf("got %d rows, want 1", len(anySlice))
	}
	if anySlice[0].(map[string]any)["name"] != "Bob" {
		t.Error("expected remaining row to be Bob")
	}
}

// --- Log-type set_items tests ---

func makeLogTree(t *testing.T) *Tree {
	t.Helper()
	root, _ := NewNode("root", TypeContainer)
	logNode, _ := NewNode("mission-log", TypeLog)
	root.Children = append(root.Children, logNode)
	logNode.parent = root

	tree, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestTree_SetItems_Log_Basic(t *testing.T) {
	tree := makeLogTree(t)
	lines := []map[string]any{
		{"id": "1", "text": "Engines nominal", "level": "info"},
		{"id": "2", "text": "Fuel pressure low", "level": "warn"},
	}
	if err := tree.SetItems("mission-log", lines); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("mission-log")
	rawLines, ok := node.GetProp("lines")
	if !ok {
		t.Fatal("lines prop not set")
	}
	anySlice, ok := rawLines.([]any)
	if !ok {
		t.Fatalf("lines prop is %T, want []any", rawLines)
	}
	if len(anySlice) != 2 {
		t.Fatalf("got %d lines, want 2", len(anySlice))
	}
	first := anySlice[0].(map[string]any)
	if first["text"] != "Engines nominal" {
		t.Errorf("first text = %q, want %q", first["text"], "Engines nominal")
	}
}

func TestTree_SetItems_Log_ReplacesExisting(t *testing.T) {
	tree := makeLogTree(t)
	if err := tree.SetItems("mission-log", []map[string]any{
		{"id": "1", "text": "old line"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tree.SetItems("mission-log", []map[string]any{
		{"id": "a", "text": "new line A"},
		{"id": "b", "text": "new line B"},
	}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("mission-log")
	anySlice := node.Props["lines"].([]any)
	if len(anySlice) != 2 {
		t.Fatalf("got %d lines, want 2", len(anySlice))
	}
	if anySlice[0].(map[string]any)["text"] != "new line A" {
		t.Error("first line not replaced")
	}
}

func TestTree_SetItems_Log_EmptyClears(t *testing.T) {
	tree := makeLogTree(t)
	if err := tree.SetItems("mission-log", []map[string]any{
		{"id": "1", "text": "line"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tree.SetItems("mission-log", nil); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("mission-log")
	anySlice := node.Props["lines"].([]any)
	if len(anySlice) != 0 {
		t.Fatalf("got %d lines, want 0", len(anySlice))
	}
}

func TestTree_AppendItems_Log_Basic(t *testing.T) {
	tree := makeLogTree(t)
	if err := tree.SetItems("mission-log", []map[string]any{
		{"id": "1", "text": "first"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tree.AppendItems("mission-log", []map[string]any{
		{"id": "2", "text": "second"},
	}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("mission-log")
	anySlice := node.Props["lines"].([]any)
	if len(anySlice) != 2 {
		t.Fatalf("got %d lines, want 2", len(anySlice))
	}
	if anySlice[1].(map[string]any)["text"] != "second" {
		t.Error("second line not appended")
	}
}

func TestTree_AppendItems_Log_ToEmpty(t *testing.T) {
	tree := makeLogTree(t)
	if err := tree.AppendItems("mission-log", []map[string]any{
		{"id": "1", "text": "first"},
	}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("mission-log")
	anySlice := node.Props["lines"].([]any)
	if len(anySlice) != 1 {
		t.Fatalf("got %d lines, want 1", len(anySlice))
	}
}

func TestTree_RemoveItems_Log_ByID(t *testing.T) {
	tree := makeLogTree(t)
	if err := tree.SetItems("mission-log", []map[string]any{
		{"id": "a", "text": "alpha"},
		{"id": "b", "text": "beta"},
		{"id": "c", "text": "charlie"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tree.RemoveItems("mission-log", []string{"b"}); err != nil {
		t.Fatal(err)
	}

	node := tree.Find("mission-log")
	anySlice := node.Props["lines"].([]any)
	if len(anySlice) != 2 {
		t.Fatalf("got %d lines, want 2", len(anySlice))
	}
	if anySlice[0].(map[string]any)["id"] != "a" {
		t.Error("expected first item to be 'a'")
	}
	if anySlice[1].(map[string]any)["id"] != "c" {
		t.Error("expected second item to be 'c'")
	}
}

func TestTree_RemoveItems_Log_Nonexistent(t *testing.T) {
	tree := makeLogTree(t)
	if err := tree.SetItems("mission-log", []map[string]any{
		{"id": "a", "text": "alpha"},
	}); err != nil {
		t.Fatal(err)
	}
	err := tree.RemoveItems("mission-log", []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}
