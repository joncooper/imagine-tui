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
