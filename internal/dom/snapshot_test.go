package dom

import (
	"testing"
)

func TestSnapshotAndRestore(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("text", "before snapshot")

	store := NewSnapshotStore()

	// Take a snapshot.
	err := store.Snapshot("v1", tree)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the tree.
	tree.Find("a1").SetProp("text", "after snapshot")
	if err := tree.Patch([]PatchOp{
		{Op: OpInsert, ParentID: "root", ID: "new_node", NodeType: TypeText},
	}); err != nil {
		t.Fatal(err)
	}

	// Verify current state is modified.
	if v, _ := tree.Find("a1").GetProp("text"); v != "after snapshot" {
		t.Fatalf("expected modified state, got %v", v)
	}
	if tree.Find("new_node") == nil {
		t.Fatal("new_node should exist before restore")
	}

	// Restore.
	err = store.Restore("v1", tree)
	if err != nil {
		t.Fatal(err)
	}

	// Verify restored state.
	if v, _ := tree.Find("a1").GetProp("text"); v != "before snapshot" {
		t.Errorf("text = %v, want 'before snapshot'", v)
	}
	if tree.Find("new_node") != nil {
		t.Error("new_node should not exist after restore")
	}
	// All original nodes should be present.
	for _, id := range []string{"root", "a", "a1", "a2", "b", "b1"} {
		if tree.Find(id) == nil {
			t.Errorf("%q missing after restore", id)
		}
	}
}

func TestSnapshotDoesNotAffectOriginalAfterMutation(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("text", "original")

	store := NewSnapshotStore()
	if err := store.Snapshot("v1", tree); err != nil {
		t.Fatal(err)
	}

	// Modify tree.
	tree.Find("a1").SetProp("text", "modified")

	// Restore and verify the snapshot preserved the original value.
	if err := store.Restore("v1", tree); err != nil {
		t.Fatal(err)
	}
	if v, _ := tree.Find("a1").GetProp("text"); v != "original" {
		t.Errorf("snapshot should have 'original', got %v", v)
	}
}

func TestRestoreNonexistentSnapshot(t *testing.T) {
	tree := makeTestTree(t)
	store := NewSnapshotStore()
	err := store.Restore("nope", tree)
	if err == nil {
		t.Fatal("expected error for nonexistent snapshot")
	}
	if !containsStr(err.Error(), "nope") {
		t.Errorf("error should mention snapshot name: %v", err)
	}
}

func TestSnapshotOverwrite(t *testing.T) {
	tree := makeTestTree(t)
	store := NewSnapshotStore()

	tree.Find("a1").SetProp("text", "version1")
	if err := store.Snapshot("v1", tree); err != nil {
		t.Fatal(err)
	}

	tree.Find("a1").SetProp("text", "version2")
	if err := store.Snapshot("v1", tree); err != nil {
		t.Fatal(err)
	}

	tree.Find("a1").SetProp("text", "version3")

	if err := store.Restore("v1", tree); err != nil {
		t.Fatal(err)
	}
	if v, _ := tree.Find("a1").GetProp("text"); v != "version2" {
		t.Errorf("should restore version2, got %v", v)
	}
}

func TestMultipleSnapshots(t *testing.T) {
	tree := makeTestTree(t)
	store := NewSnapshotStore()

	tree.Find("a1").SetProp("text", "snap_a")
	if err := store.Snapshot("snap_a", tree); err != nil {
		t.Fatal(err)
	}

	tree.Find("a1").SetProp("text", "snap_b")
	if err := store.Snapshot("snap_b", tree); err != nil {
		t.Fatal(err)
	}

	tree.Find("a1").SetProp("text", "current")

	// Restore snap_a.
	if err := store.Restore("snap_a", tree); err != nil {
		t.Fatal(err)
	}
	if v, _ := tree.Find("a1").GetProp("text"); v != "snap_a" {
		t.Errorf("expected snap_a, got %v", v)
	}

	// snap_b should still work.
	if err := store.Restore("snap_b", tree); err != nil {
		t.Fatal(err)
	}
	if v, _ := tree.Find("a1").GetProp("text"); v != "snap_b" {
		t.Errorf("expected snap_b, got %v", v)
	}
}

func TestRestoreThenModifyDoesntAffectSnapshot(t *testing.T) {
	tree := makeTestTree(t)
	store := NewSnapshotStore()

	tree.Find("a1").SetProp("text", "original")
	if err := store.Snapshot("v1", tree); err != nil {
		t.Fatal(err)
	}

	// Restore, then modify, then restore again.
	if err := store.Restore("v1", tree); err != nil {
		t.Fatal(err)
	}
	tree.Find("a1").SetProp("text", "modified_after_restore")

	// Restoring again should give the original.
	if err := store.Restore("v1", tree); err != nil {
		t.Fatal(err)
	}
	if v, _ := tree.Find("a1").GetProp("text"); v != "original" {
		t.Errorf("expected original, got %v", v)
	}
}

func TestSnapshotPreservesTreeStructure(t *testing.T) {
	tree := makeTestTree(t)
	store := NewSnapshotStore()
	if err := store.Snapshot("v1", tree); err != nil {
		t.Fatal(err)
	}

	// Remove a subtree.
	if _, err := tree.Remove("a"); err != nil {
		t.Fatal(err)
	}

	// Restore.
	if err := store.Restore("v1", tree); err != nil {
		t.Fatal(err)
	}

	// All nodes should be back.
	for _, id := range []string{"root", "a", "a1", "a2", "b", "b1"} {
		if tree.Find(id) == nil {
			t.Errorf("%q missing after restore", id)
		}
	}

	// Tree structure should be intact.
	a := tree.Find("a")
	if len(a.Children) != 2 {
		t.Errorf("a should have 2 children, got %d", len(a.Children))
	}
	if a.Parent() != tree.Root {
		t.Error("a parent should be root")
	}
}

func TestSnapshotPreservesScriptsAndComputed(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").Scripts["on_mount"] = "original_script"
	tree.Find("a1").Computed["display"] = "return 'computed'"

	store := NewSnapshotStore()
	if err := store.Snapshot("v1", tree); err != nil {
		t.Fatal(err)
	}

	tree.Find("a1").Scripts["on_mount"] = "modified"
	tree.Find("a1").Computed["display"] = "modified"

	if err := store.Restore("v1", tree); err != nil {
		t.Fatal(err)
	}
	if tree.Find("a1").Scripts["on_mount"] != "original_script" {
		t.Error("script not restored")
	}
	if tree.Find("a1").Computed["display"] != "return 'computed'" {
		t.Error("computed not restored")
	}
}

func TestSnapshotList(t *testing.T) {
	store := NewSnapshotStore()
	tree := makeTestTree(t)

	if err := store.Snapshot("a", tree); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot("b", tree); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot("c", tree); err != nil {
		t.Fatal(err)
	}

	names := store.List()
	if len(names) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(names))
	}
	// Should contain all three (order doesn't matter).
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, n := range []string{"a", "b", "c"} {
		if !nameSet[n] {
			t.Errorf("missing snapshot %q", n)
		}
	}
}
