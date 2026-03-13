package dom

import (
	"testing"
)

func TestReplaceInString(t *testing.T) {
	tests := []struct {
		name string
		s    string
		item map[string]any
		want string
	}{
		{"no placeholders", "hello", map[string]any{"k": "v"}, "hello"},
		{"single placeholder", "{{level}}", map[string]any{"level": "INFO"}, "INFO"},
		{"multiple placeholders", "{{level}}: {{msg}}", map[string]any{"level": "ERROR", "msg": "disk full"}, "ERROR: disk full"},
		{"placeholder with surrounding text", "[{{level}}] {{msg}}", map[string]any{"level": "WARN", "msg": "retry"}, "[WARN] retry"},
		{"missing key produces empty", "{{missing}}", map[string]any{"level": "INFO"}, ""},
		{"int value", "count={{n}}", map[string]any{"n": 42}, "count=42"},
		{"float value", "pct={{p}}", map[string]any{"p": 3.14}, "pct=3.14"},
		{"bool value", "ok={{b}}", map[string]any{"b": true}, "ok=true"},
		{"empty string value", "val={{e}}", map[string]any{"e": ""}, "val="},
		{"repeated placeholder", "{{x}} and {{x}}", map[string]any{"x": "A"}, "A and A"},
		{"empty item map", "{{x}}", map[string]any{}, ""},
		{"nil item map", "{{x}}", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceInString(tt.s, tt.item)
			if got != tt.want {
				t.Errorf("replaceInString(%q, %v) = %q, want %q", tt.s, tt.item, got, tt.want)
			}
		})
	}
}

func TestReplacePlaceholders(t *testing.T) {
	item := map[string]any{"level": "INFO", "msg": "started"}

	props := map[string]any{
		"content": "{{level}}",
		"width":   8,
		"bold":    true,
		"label":   "{{msg}} log",
	}
	got := replacePlaceholders(props, item)

	if got["content"] != "INFO" {
		t.Errorf("content = %v, want INFO", got["content"])
	}
	if got["width"] != 8 {
		t.Errorf("width = %v, want 8", got["width"])
	}
	if got["bold"] != true {
		t.Errorf("bold = %v, want true", got["bold"])
	}
	if got["label"] != "started log" {
		t.Errorf("label = %v, want 'started log'", got["label"])
	}
}

func TestReplacePlaceholders_NilProps(t *testing.T) {
	got := replacePlaceholders(nil, map[string]any{"k": "v"})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExpandTemplate_SingleItem(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{msg}}"},
	}
	items := []map[string]any{
		{"msg": "hello"},
	}

	specs, err := ExpandTemplate("list", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("got %d specs, want 1", len(specs))
	}
	s := specs[0]
	if s.ID != "list-0" {
		t.Errorf("ID = %q, want %q", s.ID, "list-0")
	}
	if s.Type != TypeText {
		t.Errorf("Type = %q, want %q", s.Type, TypeText)
	}
	if s.Props["content"] != "hello" {
		t.Errorf("content = %v, want %q", s.Props["content"], "hello")
	}
}

func TestExpandTemplate_MultipleItems(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{msg}}"},
	}
	items := []map[string]any{
		{"msg": "one"},
		{"msg": "two"},
		{"msg": "three"},
	}

	specs, err := ExpandTemplate("list", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 3 {
		t.Fatalf("got %d specs, want 3", len(specs))
	}
	for i, want := range []string{"one", "two", "three"} {
		if specs[i].Props["content"] != want {
			t.Errorf("specs[%d].content = %v, want %q", i, specs[i].Props["content"], want)
		}
		wantID := "list-" + string(rune('0'+i))
		if specs[i].ID != wantID {
			t.Errorf("specs[%d].ID = %q, want %q", i, specs[i].ID, wantID)
		}
	}
}

func TestExpandTemplate_WithKeyField(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{msg}}"},
	}
	items := []map[string]any{
		{"key": "err1", "msg": "disk full"},
		{"key": "err2", "msg": "timeout"},
	}

	specs, err := ExpandTemplate("log", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(specs))
	}
	if specs[0].ID != "log-err1" {
		t.Errorf("specs[0].ID = %q, want %q", specs[0].ID, "log-err1")
	}
	if specs[1].ID != "log-err2" {
		t.Errorf("specs[1].ID = %q, want %q", specs[1].ID, "log-err2")
	}
}

func TestExpandTemplate_NestedChildren(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeContainer,
		Props: map[string]any{"direction": "row"},
		Children: []*NodeSpec{
			{Type: TypeText, Props: map[string]any{"content": "{{level}}"}},
			{Type: TypeText, Props: map[string]any{"content": "{{msg}}"}},
		},
	}
	items := []map[string]any{
		{"level": "INFO", "msg": "started"},
	}

	specs, err := ExpandTemplate("log", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("got %d specs, want 1", len(specs))
	}

	root := specs[0]
	if root.ID != "log-0" {
		t.Errorf("root.ID = %q, want %q", root.ID, "log-0")
	}
	if len(root.Children) != 2 {
		t.Fatalf("root has %d children, want 2", len(root.Children))
	}
	if root.Children[0].ID != "log-0-0" {
		t.Errorf("child[0].ID = %q, want %q", root.Children[0].ID, "log-0-0")
	}
	if root.Children[0].Props["content"] != "INFO" {
		t.Errorf("child[0].content = %v, want %q", root.Children[0].Props["content"], "INFO")
	}
	if root.Children[1].ID != "log-0-1" {
		t.Errorf("child[1].ID = %q, want %q", root.Children[1].ID, "log-0-1")
	}
	if root.Children[1].Props["content"] != "started" {
		t.Errorf("child[1].content = %v, want %q", root.Children[1].Props["content"], "started")
	}
}

func TestExpandTemplate_MissingPlaceholder(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{missing}}"},
	}
	items := []map[string]any{
		{"other": "value"},
	}

	specs, err := ExpandTemplate("list", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].Props["content"] != "" {
		t.Errorf("content = %v, want empty string", specs[0].Props["content"])
	}
}

func TestExpandTemplate_NilTemplate(t *testing.T) {
	_, err := ExpandTemplate("list", nil, []map[string]any{{"k": "v"}})
	if err == nil {
		t.Fatal("expected error for nil template")
	}
}

func TestExpandTemplate_EmptyItems(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{msg}}"},
	}
	specs, err := ExpandTemplate("list", tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Errorf("got %d specs, want 0", len(specs))
	}

	specs, err = ExpandTemplate("list", tmpl, []map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Errorf("got %d specs, want 0", len(specs))
	}
}

func TestExpandTemplate_NonStringProps(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{msg}}", "width": 8, "bold": true},
	}
	items := []map[string]any{
		{"msg": "hello"},
	}

	specs, err := ExpandTemplate("list", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].Props["width"] != 8 {
		t.Errorf("width = %v, want 8", specs[0].Props["width"])
	}
	if specs[0].Props["bold"] != true {
		t.Errorf("bold = %v, want true", specs[0].Props["bold"])
	}
}

func TestExpandTemplate_PreservesNestedItemTemplate(t *testing.T) {
	// A template containing a child with its own item_template should preserve it as data.
	innerTemplate := map[string]any{
		"type":  "text",
		"props": map[string]any{"content": "{{inner}}"},
	}
	tmpl := &NodeSpec{
		Type: TypeContainer,
		Props: map[string]any{
			"item_template": innerTemplate,
		},
	}
	items := []map[string]any{
		{"k": "v"},
	}

	specs, err := ExpandTemplate("list", tmpl, items)
	if err != nil {
		t.Fatal(err)
	}
	// The item_template prop should be preserved (not expanded).
	it, ok := specs[0].Props["item_template"]
	if !ok {
		t.Fatal("item_template prop was removed")
	}
	itMap, ok := it.(map[string]any)
	if !ok {
		t.Fatalf("item_template is %T, want map[string]any", it)
	}
	if itMap["type"] != "text" {
		t.Errorf("nested item_template type = %v, want text", itMap["type"])
	}
}

func TestParseItemTemplate(t *testing.T) {
	node, err := NewNode("list", TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	node.SetProp("item_template", map[string]any{
		"type": "text",
		"props": map[string]any{
			"content": "{{msg}}",
		},
	})

	spec, err := ParseItemTemplate(node)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Type != TypeText {
		t.Errorf("Type = %q, want %q", spec.Type, TypeText)
	}
	if spec.Props["content"] != "{{msg}}" {
		t.Errorf("content = %v, want {{msg}}", spec.Props["content"])
	}
}

func TestParseItemTemplate_WithChildren(t *testing.T) {
	node, err := NewNode("list", TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	node.SetProp("item_template", map[string]any{
		"type": "container",
		"children": []any{
			map[string]any{"type": "text", "props": map[string]any{"content": "{{a}}"}},
			map[string]any{"type": "text", "props": map[string]any{"content": "{{b}}"}},
		},
	})

	spec, err := ParseItemTemplate(node)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(spec.Children))
	}
}

func TestParseItemTemplate_Missing(t *testing.T) {
	node, err := NewNode("list", TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseItemTemplate(node)
	if err == nil {
		t.Fatal("expected error for missing item_template")
	}
}

func TestParseItemTemplate_Invalid(t *testing.T) {
	node, err := NewNode("list", TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	node.SetProp("item_template", "not a map")
	_, err = ParseItemTemplate(node)
	if err == nil {
		t.Fatal("expected error for non-map item_template")
	}
}

func TestExpandTemplate_OffsetIndex(t *testing.T) {
	tmpl := &NodeSpec{
		Type:  TypeText,
		Props: map[string]any{"content": "{{msg}}"},
	}
	items := []map[string]any{
		{"msg": "a"},
		{"msg": "b"},
	}

	// ExpandTemplateAt with offset=3 should produce IDs list-3, list-4
	specs, err := ExpandTemplateAt("list", tmpl, items, 3)
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].ID != "list-3" {
		t.Errorf("specs[0].ID = %q, want %q", specs[0].ID, "list-3")
	}
	if specs[1].ID != "list-4" {
		t.Errorf("specs[1].ID = %q, want %q", specs[1].ID, "list-4")
	}
}
