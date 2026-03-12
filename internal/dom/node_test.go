package dom

import (
	"testing"
)

func TestNewNode(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		nodeType NodeType
		wantErr  bool
		errMsg   string
	}{
		{name: "valid container", id: "root", nodeType: TypeContainer},
		{name: "valid text", id: "header", nodeType: TypeText},
		{name: "valid input", id: "search_input", nodeType: TypeInput},
		{name: "valid button", id: "submit-btn", nodeType: TypeButton},
		{name: "valid with dots", id: "section.1", nodeType: TypeContainer},
		{name: "empty ID", id: "", nodeType: TypeContainer, wantErr: true, errMsg: "must not be empty"},
		{name: "whitespace in ID", id: "bad id", nodeType: TypeContainer, wantErr: true, errMsg: "must not contain whitespace"},
		{name: "tab in ID", id: "bad\tid", nodeType: TypeContainer, wantErr: true, errMsg: "must not contain whitespace"},
		{name: "newline in ID", id: "bad\nid", nodeType: TypeContainer, wantErr: true, errMsg: "must not contain whitespace"},
		{name: "unknown type", id: "node1", nodeType: "sparkline", wantErr: true, errMsg: "unknown type"},
		{name: "empty type", id: "node1", nodeType: "", wantErr: true, errMsg: "unknown type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNode(tt.id, tt.nodeType)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n.ID != tt.id {
				t.Errorf("ID = %q, want %q", n.ID, tt.id)
			}
			if n.Type != tt.nodeType {
				t.Errorf("Type = %q, want %q", n.Type, tt.nodeType)
			}
			if n.Props == nil {
				t.Error("Props map should be initialized")
			}
			if n.Scripts == nil {
				t.Error("Scripts map should be initialized")
			}
			if n.Computed == nil {
				t.Error("Computed map should be initialized")
			}
			if n.Children != nil {
				t.Error("Children should be nil initially")
			}
			if n.Parent() != nil {
				t.Error("Parent should be nil for new node")
			}
		})
	}
}

func TestNodeProps(t *testing.T) {
	n, err := NewNode("test", TypeText)
	if err != nil {
		t.Fatal(err)
	}

	// Set and get a prop.
	n.SetProp("text", "hello")
	v, ok := n.GetProp("text")
	if !ok {
		t.Fatal("expected prop to exist")
	}
	if v != "hello" {
		t.Errorf("got %v, want hello", v)
	}

	// Get nonexistent prop.
	_, ok = n.GetProp("nonexistent")
	if ok {
		t.Error("expected prop to not exist")
	}
}

func TestNodeDeepCopy(t *testing.T) {
	parent, _ := NewNode("parent", TypeContainer)
	child, _ := NewNode("child", TypeText)
	child.SetProp("text", "hello")
	child.Scripts["on_mount"] = "console.log('hi')"
	child.Computed["display"] = "return $.value"
	child.parent = parent
	parent.Children = append(parent.Children, child)

	// Deep copy the parent.
	cp := parent.deepCopy()

	// Verify structure.
	if cp.ID != "parent" {
		t.Errorf("copy ID = %q, want parent", cp.ID)
	}
	if len(cp.Children) != 1 {
		t.Fatalf("copy children = %d, want 1", len(cp.Children))
	}
	if cp.Children[0].ID != "child" {
		t.Errorf("child copy ID = %q, want child", cp.Children[0].ID)
	}
	if cp.Children[0].Parent() != cp {
		t.Error("child copy parent should point to copied parent")
	}
	if cp.Parent() != nil {
		t.Error("copied root parent should be nil")
	}

	// Verify independence - mutating copy doesn't affect original.
	cp.Children[0].SetProp("text", "modified")
	origText, _ := child.GetProp("text")
	if origText != "hello" {
		t.Errorf("original was modified: got %v, want hello", origText)
	}

	cp.Children[0].Scripts["on_mount"] = "modified"
	if child.Scripts["on_mount"] != "console.log('hi')" {
		t.Error("original scripts were modified")
	}
}

func TestIsValidType(t *testing.T) {
	valid := []NodeType{TypeContainer, TypeText, TypeInput, TypeTextarea, TypeSelect,
		TypeButton, TypeTable, TypeList, TypeDiff, TypeLog, TypeCode,
		TypeTabs, TypeProgress, TypeTree, TypeModal, TypeForm}
	for _, vt := range valid {
		if !IsValidType(vt) {
			t.Errorf("expected %q to be valid", vt)
		}
	}
	invalid := []NodeType{"", "sparkline", "custom", "div"}
	for _, ivt := range invalid {
		if IsValidType(ivt) {
			t.Errorf("expected %q to be invalid", ivt)
		}
	}
}

func TestDeepCopyNestedProps(t *testing.T) {
	n, _ := NewNode("test", TypeTable)
	n.SetProp("rows", []any{
		map[string]any{"name": "Alice", "age": 30},
		map[string]any{"name": "Bob", "age": 25},
	})

	cp := n.deepCopy()

	// Mutate the copy's nested data.
	rows := cp.Props["rows"].([]any)
	rows[0].(map[string]any)["name"] = "Charlie"

	// Original should be unaffected.
	origRows := n.Props["rows"].([]any)
	if origRows[0].(map[string]any)["name"] != "Alice" {
		t.Error("original nested prop was modified by copy mutation")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
