package dom

import (
	"encoding/json"
	"testing"
)

func TestPatchUpdateSingle(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("text", "original")

	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "a1", Props: map[string]any{"text": "updated", "color": "red"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Find("a1")
	if v, _ := n.GetProp("text"); v != "updated" {
		t.Errorf("text = %v, want updated", v)
	}
	if v, _ := n.GetProp("color"); v != "red" {
		t.Errorf("color = %v, want red", v)
	}
}

func TestPatchUpdateMergesProps(t *testing.T) {
	tree := makeTestTree(t)
	n := tree.Find("a1")
	n.SetProp("text", "hello")
	n.SetProp("style", "bold")

	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "a1", Props: map[string]any{"text": "updated"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// style should be preserved (merge, not replace).
	if v, _ := n.GetProp("style"); v != "bold" {
		t.Errorf("style = %v, want bold (should be preserved)", v)
	}
	if v, _ := n.GetProp("text"); v != "updated" {
		t.Errorf("text = %v, want updated", v)
	}
}

func TestPatchUpdateScriptsAndComputed(t *testing.T) {
	tree := makeTestTree(t)
	n := tree.Find("a1")
	n.Scripts["on_mount"] = "original"
	n.Computed["display"] = "return 'hi'"

	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "a1",
			Scripts:  map[string]string{"on_change": "new script"},
			Computed: map[string]string{"summary": "return 'sum'"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Original scripts/computed preserved, new ones added.
	if n.Scripts["on_mount"] != "original" {
		t.Error("original script lost")
	}
	if n.Scripts["on_change"] != "new script" {
		t.Error("new script not added")
	}
	if n.Computed["display"] != "return 'hi'" {
		t.Error("original computed lost")
	}
	if n.Computed["summary"] != "return 'sum'" {
		t.Error("new computed not added")
	}
}

func TestPatchUpdateNonexistent(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "nope", Props: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	pe, ok := err.(*PatchError)
	if !ok {
		t.Fatalf("expected PatchError, got %T", err)
	}
	if pe.OpIndex != 0 {
		t.Errorf("OpIndex = %d, want 0", pe.OpIndex)
	}
}

func TestPatchUpdateEmptyID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "", Props: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestPatchInsertFlat(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "c", NodeType: TypeText,
			Props: map[string]any{"text": "new"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := tree.Find("c")
	if c == nil {
		t.Fatal("inserted node not found")
	}
	if c.Type != TypeText {
		t.Errorf("type = %q, want text", c.Type)
	}
	if v, _ := c.GetProp("text"); v != "new" {
		t.Errorf("text = %v, want new", v)
	}
}

func TestPatchInsertWithSpec(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", Node: &NodeSpec{
			ID:   "panel",
			Type: TypeContainer,
			Children: []*NodeSpec{
				{ID: "panel_text", Type: TypeText, Props: map[string]any{"text": "hello"}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Find("panel") == nil {
		t.Error("panel not found")
	}
	if tree.Find("panel_text") == nil {
		t.Error("panel_text not found")
	}
}

func TestPatchInsertAfterSibling(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "a", ID: "a1.5", NodeType: TypeText, AfterID: "a1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a := tree.Find("a")
	if len(a.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(a.Children))
	}
	if a.Children[1].ID != "a1.5" {
		t.Errorf("child[1] = %q, want a1.5", a.Children[1].ID)
	}
}

func TestPatchInsertDuplicateID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "a", NodeType: TypeText},
	})
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestPatchInsertMissingParent(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "nope", ID: "x", NodeType: TypeText},
	})
	if err == nil {
		t.Fatal("expected error for missing parent")
	}
}

func TestPatchInsertMissingType(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "x"},
	})
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestPatchRemove(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpRemove, ID: "b1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Find("b1") != nil {
		t.Error("b1 still in tree")
	}
}

func TestPatchRemoveSubtree(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpRemove, ID: "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "a1", "a2"} {
		if tree.Find(id) != nil {
			t.Errorf("%q still in tree", id)
		}
	}
}

func TestPatchRemoveNonexistent(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpRemove, ID: "nope"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPatchMove(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpMove, ID: "b1", ParentID: "a", AfterID: "a1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b1 := tree.Find("b1")
	if b1.Parent().ID != "a" {
		t.Errorf("b1 parent = %q, want a", b1.Parent().ID)
	}
}

func TestPatchMoveCycle(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpMove, ID: "a", ParentID: "a1"},
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestPatchMultiOp(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "c", NodeType: TypeText},
		{Op: OpUpdate, ID: "c", Props: map[string]any{"text": "hello"}},
		{Op: OpMove, ID: "c", ParentID: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := tree.Find("c")
	if c == nil {
		t.Fatal("c not found")
	}
	if c.Parent().ID != "b" {
		t.Errorf("c parent = %q, want b", c.Parent().ID)
	}
	if v, _ := c.GetProp("text"); v != "hello" {
		t.Errorf("text = %v, want hello", v)
	}
}

func TestPatchRollbackOnFailure(t *testing.T) {
	tree := makeTestTree(t)
	// First op succeeds, second fails — everything should roll back.
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "temp", NodeType: TypeText},
		{Op: OpUpdate, ID: "nonexistent", Props: map[string]any{"text": "x"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*PatchError)
	if !ok {
		t.Fatalf("expected PatchError, got %T", err)
	}
	if pe.OpIndex != 1 {
		t.Errorf("OpIndex = %d, want 1", pe.OpIndex)
	}
	// "temp" should not exist (rolled back).
	if tree.Find("temp") != nil {
		t.Error("temp node exists after rollback — atomicity violated")
	}
}

func TestPatchRollbackPreservesOriginalState(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("text", "original")

	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "a1", Props: map[string]any{"text": "modified"}},
		{Op: OpRemove, ID: "nonexistent"}, // fails
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// a1's text should be rolled back.
	if v, _ := tree.Find("a1").GetProp("text"); v != "original" {
		t.Errorf("text = %v after rollback, want original", v)
	}
}

func TestPatchEmptyOps(t *testing.T) {
	tree := makeTestTree(t)
	if err := tree.Patch(nil); err != nil {
		t.Fatal(err)
	}
	if err := tree.Patch([]PatchOp{}); err != nil {
		t.Fatal(err)
	}
}

func TestPatchUnknownOp(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: "unknown_op"},
	})
	if err == nil {
		t.Fatal("expected error for unknown op")
	}
}

func TestParsePatchOps(t *testing.T) {
	raw := `[
		{"op":"update","id":"a1","props":{"text":"hello"}},
		{"op":"insert","parent_id":"root","id":"c","type":"text"},
		{"op":"remove","id":"b1"},
		{"op":"move","id":"a2","parent_id":"b"}
	]`
	ops, err := ParsePatchOps([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 4 {
		t.Fatalf("expected 4 ops, got %d", len(ops))
	}
	if ops[0].Op != OpUpdate || ops[0].ID != "a1" {
		t.Errorf("op[0] = %+v", ops[0])
	}
	if ops[1].Op != OpInsert || ops[1].ParentID != "root" {
		t.Errorf("op[1] = %+v", ops[1])
	}
	if ops[2].Op != OpRemove || ops[2].ID != "b1" {
		t.Errorf("op[2] = %+v", ops[2])
	}
	if ops[3].Op != OpMove || ops[3].ParentID != "b" {
		t.Errorf("op[3] = %+v", ops[3])
	}
}

func TestParsePatchOpsInvalidJSON(t *testing.T) {
	_, err := ParsePatchOps([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePatchOpsWithNodeSpec(t *testing.T) {
	raw := `[{"op":"insert","parent_id":"root","node":{"id":"panel","type":"container","children":[{"id":"txt","type":"text","props":{"text":"hi"}}]}}]`
	ops, err := ParsePatchOps([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Node == nil {
		t.Fatal("node spec is nil")
	}
	if ops[0].Node.ID != "panel" {
		t.Errorf("node ID = %q", ops[0].Node.ID)
	}
	if len(ops[0].Node.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(ops[0].Node.Children))
	}
}

func TestPatchInsertThenUpdateInSameBatch(t *testing.T) {
	// An insert creates a node, and a subsequent update in the same batch
	// references the newly created node. This tests forward references.
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "new_node", NodeType: TypeText},
		{Op: OpUpdate, ID: "new_node", Props: map[string]any{"text": "set after insert"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Find("new_node")
	if v, _ := n.GetProp("text"); v != "set after insert" {
		t.Errorf("text = %v, want 'set after insert'", v)
	}
}

func TestPatchRemoveThenInsertSameID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpRemove, ID: "a1"},
		{Op: OpInsert, ParentID: "b", ID: "a1", NodeType: TypeButton},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Find("a1")
	if n == nil {
		t.Fatal("a1 not found after remove+insert")
	}
	if n.Type != TypeButton {
		t.Errorf("type = %q, want button", n.Type)
	}
	if n.Parent().ID != "b" {
		t.Errorf("parent = %q, want b", n.Parent().ID)
	}
}

func TestPatchErrorFormat(t *testing.T) {
	pe := &PatchError{OpIndex: 2, Op: OpUpdate, Message: "node \"x\" not found"}
	got := pe.Error()
	if got != `patch op[2] update: node "x" not found` {
		t.Errorf("Error() = %q", got)
	}
}

func TestPatchRemoveEmptyID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpRemove, ID: ""},
	})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestPatchMoveEmptyID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpMove, ID: "", ParentID: "root"},
	})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestPatchMoveMissingParentID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpMove, ID: "b1", ParentID: ""},
	})
	if err == nil {
		t.Fatal("expected error for empty parent_id")
	}
}

func TestPatchInsertMissingParentID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "", ID: "x", NodeType: TypeText},
	})
	if err == nil {
		t.Fatal("expected error for empty parent_id")
	}
}

func TestPatchInsertMissingID(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "", NodeType: TypeText},
	})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestPatchInsertFlatWithScriptsAndComputed(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "scripted", NodeType: TypeText,
			Props:    map[string]any{"text": "hi"},
			Scripts:  map[string]string{"on_mount": "init()"},
			Computed: map[string]string{"display": "return 'x'"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Find("scripted")
	if n.Scripts["on_mount"] != "init()" {
		t.Error("scripts not set on flat insert")
	}
	if n.Computed["display"] != "return 'x'" {
		t.Error("computed not set on flat insert")
	}
}

func TestPatchInsertSpecWithScriptsAndComputed(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", Node: &NodeSpec{
			ID:       "s_node",
			Type:     TypeButton,
			Scripts:  map[string]string{"on_click": "handleClick()"},
			Computed: map[string]string{"label": "return 'Go'"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Find("s_node")
	if n.Scripts["on_click"] != "handleClick()" {
		t.Error("scripts not set on spec insert")
	}
	if n.Computed["label"] != "return 'Go'" {
		t.Error("computed not set on spec insert")
	}
}

func TestPatchInsertInvalidType(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "bad", NodeType: "sparkline"},
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestPatchInsertSpecInvalidType(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", Node: &NodeSpec{
			ID: "bad", Type: "sparkline",
		}},
	})
	if err == nil {
		t.Fatal("expected error for invalid type in spec")
	}
}

func TestPatchDuplicateUpdateSameNode(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "a1", Props: map[string]any{"text": "first"}},
		{Op: OpUpdate, ID: "a1", Props: map[string]any{"text": "second", "color": "blue"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Find("a1")
	if v, _ := n.GetProp("text"); v != "second" {
		t.Errorf("text = %v, want second (last write wins)", v)
	}
	if v, _ := n.GetProp("color"); v != "blue" {
		t.Errorf("color = %v, want blue", v)
	}
}

func TestPatchRollbackOpIndex(t *testing.T) {
	tree := makeTestTree(t)
	// Insert succeeds, insert succeeds, move fails (cycle) — index should be 2.
	err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "c1", NodeType: TypeText},
		{Op: OpInsert, ParentID: "root", ID: "c2", NodeType: TypeText},
		{Op: OpMove, ID: "a", ParentID: "a1"}, // cycle
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe := err.(*PatchError)
	if pe.OpIndex != 2 {
		t.Errorf("OpIndex = %d, want 2", pe.OpIndex)
	}
	if pe.Op != OpMove {
		t.Errorf("Op = %q, want move", pe.Op)
	}
	// c1 and c2 should be rolled back.
	if tree.Find("c1") != nil || tree.Find("c2") != nil {
		t.Error("rolled-back nodes still in tree")
	}
}

func TestPatchUpdateOverwriteScript(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").Scripts["on_mount"] = "old"

	if err := tree.Patch([]PatchOp{
		{Op: OpUpdate, ID: "a1", Scripts: map[string]string{"on_mount": "new"}},
	}); err != nil {
		t.Fatal(err)
	}
	if tree.Find("a1").Scripts["on_mount"] != "new" {
		t.Error("script overwrite failed")
	}
}

func TestPatchMoveToRoot(t *testing.T) {
	tree := makeTestTree(t)
	err := tree.Patch([]PatchOp{
		{Op: OpMove, ID: "b1", ParentID: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Find("b1").Parent().ID != "root" {
		t.Error("b1 should be child of root")
	}
}

// Test JSON round-trip for PatchOp serialization.
func TestPatchOpJSON(t *testing.T) {
	ops := []PatchOp{
		{Op: OpUpdate, ID: "x", Props: map[string]any{"text": "hi"}},
		{Op: OpInsert, ParentID: "root", ID: "y", NodeType: TypeText},
	}
	data, err := json.Marshal(ops)
	if err != nil {
		t.Fatal(err)
	}
	var parsed []PatchOp
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(parsed))
	}
	if parsed[0].ID != "x" || parsed[1].ID != "y" {
		t.Error("round-trip mismatch")
	}
}
