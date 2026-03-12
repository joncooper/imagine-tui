package dom

import (
	"testing"
)

func TestReplaceChildren(t *testing.T) {
	t.Run("replace all children of a node", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.Replace("a", []*NodeSpec{
			{ID: "x1", Type: TypeText, Props: map[string]any{"text": "new1"}},
			{ID: "x2", Type: TypeButton},
		})
		if err != nil {
			t.Fatal(err)
		}
		a := tree.Find("a")
		if len(a.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(a.Children))
		}
		if a.Children[0].ID != "x1" || a.Children[1].ID != "x2" {
			t.Errorf("children = [%s, %s], want [x1, x2]", a.Children[0].ID, a.Children[1].ID)
		}
		// Old children should be gone from the index.
		if tree.Find("a1") != nil {
			t.Error("a1 still in index after replace")
		}
		if tree.Find("a2") != nil {
			t.Error("a2 still in index after replace")
		}
		// New children should be in the index.
		if tree.Find("x1") == nil {
			t.Error("x1 not in index")
		}
		if tree.Find("x2") == nil {
			t.Error("x2 not in index")
		}
		// Parent pointers.
		if tree.Find("x1").Parent() != a {
			t.Error("x1 parent is wrong")
		}
	})

	t.Run("replace with nested subtree", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.Replace("root", []*NodeSpec{
			{ID: "panel", Type: TypeContainer, Children: []*NodeSpec{
				{ID: "header", Type: TypeText, Props: map[string]any{"text": "Title"}},
				{ID: "body", Type: TypeContainer, Children: []*NodeSpec{
					{ID: "content", Type: TypeText},
				}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		// All old children gone.
		for _, id := range []string{"a", "a1", "a2", "b", "b1"} {
			if tree.Find(id) != nil {
				t.Errorf("%q still in index", id)
			}
		}
		// All new nodes present.
		for _, id := range []string{"panel", "header", "body", "content"} {
			if tree.Find(id) == nil {
				t.Errorf("%q not in index", id)
			}
		}
	})

	t.Run("replace with empty children (clear)", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.Replace("a", []*NodeSpec{})
		if err != nil {
			t.Fatal(err)
		}
		a := tree.Find("a")
		if len(a.Children) != 0 {
			t.Errorf("expected 0 children, got %d", len(a.Children))
		}
	})

	t.Run("replace nonexistent node fails", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.Replace("nope", []*NodeSpec{
			{ID: "x", Type: TypeText},
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ID collision with existing tree fails", func(t *testing.T) {
		tree := makeTestTree(t)
		// b1 exists outside of a's subtree, so using it as a new child of a should fail.
		err := tree.Replace("a", []*NodeSpec{
			{ID: "b1", Type: TypeText},
		})
		if err == nil {
			t.Fatal("expected duplicate ID error")
		}
	})

	t.Run("ID collision with target fails", func(t *testing.T) {
		tree := makeTestTree(t)
		// New subtree contains "a" which is the target itself.
		err := tree.Replace("a", []*NodeSpec{
			{ID: "a", Type: TypeText},
		})
		if err == nil {
			t.Fatal("expected error for ID collision with target")
		}
	})

	t.Run("reusing removed IDs is ok", func(t *testing.T) {
		tree := makeTestTree(t)
		// Replace a's children with nodes that reuse the old child IDs.
		err := tree.Replace("a", []*NodeSpec{
			{ID: "a1", Type: TypeButton},
			{ID: "a2", Type: TypeInput},
		})
		if err != nil {
			t.Fatal(err)
		}
		// The new a1 should be a button now.
		if tree.Find("a1").Type != TypeButton {
			t.Errorf("a1 type = %q, want button", tree.Find("a1").Type)
		}
	})

	t.Run("duplicate IDs within new subtree fails", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.Replace("a", []*NodeSpec{
			{ID: "dup", Type: TypeText},
			{ID: "dup", Type: TypeButton},
		})
		if err == nil {
			t.Fatal("expected error for duplicate IDs within new subtree")
		}
	})
}

func TestReplaceTree(t *testing.T) {
	t.Run("full tree replacement", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.ReplaceTree(&NodeSpec{
			ID:   "new_root",
			Type: TypeContainer,
			Children: []*NodeSpec{
				{ID: "child1", Type: TypeText},
				{ID: "child2", Type: TypeButton},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if tree.Root.ID != "new_root" {
			t.Errorf("root ID = %q, want new_root", tree.Root.ID)
		}
		if tree.Find("new_root") == nil {
			t.Error("new_root not in index")
		}
		if tree.Find("child1") == nil || tree.Find("child2") == nil {
			t.Error("children not in index")
		}
		// Old nodes gone.
		if tree.Find("root") != nil {
			t.Error("old root still in index")
		}
	})

	t.Run("replace tree with duplicate IDs fails", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.ReplaceTree(&NodeSpec{
			ID:   "r",
			Type: TypeContainer,
			Children: []*NodeSpec{
				{ID: "r", Type: TypeText}, // conflicts with root
			},
		})
		if err == nil {
			t.Fatal("expected error for duplicate IDs")
		}
	})

	t.Run("replace tree with invalid type fails", func(t *testing.T) {
		tree := makeTestTree(t)
		err := tree.ReplaceTree(&NodeSpec{
			ID:   "r",
			Type: "invalid_type",
		})
		if err == nil {
			t.Fatal("expected error for invalid type")
		}
	})
}
