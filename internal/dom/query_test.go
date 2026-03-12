package dom

import (
	"testing"
)

func TestQuerySingleNode(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("text", "hello")
	tree.Find("a1").SetProp("style", "bold")

	results, errs := tree.Query([]string{"a1"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	r, ok := results["a1"]
	if !ok {
		t.Fatal("a1 not in results")
	}
	if r.ID != "a1" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Type != TypeText {
		t.Errorf("Type = %q", r.Type)
	}
	if r.Props["text"] != "hello" {
		t.Errorf("text = %v", r.Props["text"])
	}
	if r.Props["style"] != "bold" {
		t.Errorf("style = %v", r.Props["style"])
	}
}

func TestQueryMultipleNodes(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").SetProp("text", "first")
	tree.Find("b1").SetProp("label", "click me")

	results, errs := tree.Query([]string{"a1", "b1"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results["a1"].Props["text"] != "first" {
		t.Error("a1 text wrong")
	}
	if results["b1"].Props["label"] != "click me" {
		t.Error("b1 label wrong")
	}
}

func TestQueryNonexistentNode(t *testing.T) {
	tree := makeTestTree(t)
	results, errs := tree.Query([]string{"a1", "nonexistent"})
	// Should return partial results + errors for missing.
	if _, ok := results["a1"]; !ok {
		t.Error("a1 should be in results")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !containsStr(errs[0].Error(), "nonexistent") {
		t.Errorf("error should mention nonexistent: %v", errs[0])
	}
}

func TestQueryAllNonexistent(t *testing.T) {
	tree := makeTestTree(t)
	results, errs := tree.Query([]string{"nope1", "nope2"})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}
}

func TestQueryEmptyList(t *testing.T) {
	tree := makeTestTree(t)
	results, errs := tree.Query([]string{})
	if len(results) != 0 {
		t.Error("expected empty results")
	}
	if len(errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestQueryIncludesComputed(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").Computed["display"] = "return 'computed_value'"

	results, errs := tree.Query([]string{"a1"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	r := results["a1"]
	if r.Computed["display"] != "return 'computed_value'" {
		t.Errorf("computed = %v", r.Computed)
	}
}

func TestQueryIncludesScripts(t *testing.T) {
	tree := makeTestTree(t)
	tree.Find("a1").Scripts["on_mount"] = "console.log('hi')"

	results, errs := tree.Query([]string{"a1"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	r := results["a1"]
	if r.Scripts["on_mount"] != "console.log('hi')" {
		t.Errorf("scripts = %v", r.Scripts)
	}
}

func TestQueryIncludesChildrenIDs(t *testing.T) {
	tree := makeTestTree(t)
	results, errs := tree.Query([]string{"a"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	r := results["a"]
	if len(r.ChildIDs) != 2 {
		t.Fatalf("expected 2 child IDs, got %d", len(r.ChildIDs))
	}
	if r.ChildIDs[0] != "a1" || r.ChildIDs[1] != "a2" {
		t.Errorf("child IDs = %v", r.ChildIDs)
	}
}

func TestQueryIncludesParentID(t *testing.T) {
	tree := makeTestTree(t)
	results, errs := tree.Query([]string{"a1"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if results["a1"].ParentID != "a" {
		t.Errorf("parent ID = %q, want a", results["a1"].ParentID)
	}
}

func TestQueryRootHasNoParent(t *testing.T) {
	tree := makeTestTree(t)
	results, errs := tree.Query([]string{"root"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if results["root"].ParentID != "" {
		t.Errorf("root parent ID = %q, want empty", results["root"].ParentID)
	}
}
