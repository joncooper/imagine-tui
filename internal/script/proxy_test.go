package script

import (
	"errors"
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

// newTestRuntimeWithTree creates a runtime with a richer tree for proxy tests:
// root (container)
//
//	├── child1 (text, props: {text: "hello", style: "bold", visible: true})
//	└── child2 (input, props: {value: "world", placeholder: "type..."})
func newTestRuntimeWithTree(t *testing.T) *Runtime {
	t.Helper()
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}

	child1, err := dom.NewNode("child1", dom.TypeText)
	if err != nil {
		t.Fatal(err)
	}
	child1.SetProp("text", "hello")
	child1.SetProp("style", "bold")
	child1.SetProp("visible", true)

	child2, err := dom.NewNode("child2", dom.TypeInput)
	if err != nil {
		t.Fatal(err)
	}
	child2.SetProp("value", "world")
	child2.SetProp("placeholder", "type...")

	if err := tree.Insert("root", child1, ""); err != nil {
		t.Fatal(err)
	}
	if err := tree.Insert("root", child2, ""); err != nil {
		t.Fatal(err)
	}

	events := dom.NewEventQueue()
	return New(tree, events)
}

func TestProxyReadID(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.id !== "child1") {
			throw new Error("expected id='child1', got " + $.id);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadType(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.type !== "text") {
			throw new Error("expected type='text', got " + $.type);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadValue(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child2", "test", `
		if ($.value !== "world") {
			throw new Error("expected value='world', got " + $.value);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadText(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.text !== "hello") {
			throw new Error("expected text='hello', got " + $.text);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadStyle(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.style !== "bold") {
			throw new Error("expected style='bold', got " + $.style);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadVisible(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.visible !== true) {
			throw new Error("expected visible=true, got " + $.visible);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadVisibleDefault(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// child2 doesn't have visible set — should default to true.
	err := rt.execScript("child2", "test", `
		if ($.visible !== true) {
			throw new Error("expected visible=true (default), got " + $.visible);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadUnknownPropFallsThrough(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child2", "test", `
		if ($.placeholder !== "type...") {
			throw new Error("expected placeholder='type...', got " + $.placeholder);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadUndefinedProp(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.nonexistent !== undefined) {
			throw new Error("expected undefined, got " + $.nonexistent);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyWriteValue(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child2", "test", `$.value = "updated"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child2")
	v, ok := node.GetProp("value")
	if !ok || v != "updated" {
		t.Errorf("expected DOM value='updated', got %v", v)
	}
}

func TestProxyWriteText(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `$.text = "goodbye"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	v, _ := node.GetProp("text")
	if v != "goodbye" {
		t.Errorf("expected DOM text='goodbye', got %v", v)
	}
}

func TestProxyWriteVisible(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `$.visible = false`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	v, ok := node.GetProp("visible")
	if !ok || v != false {
		t.Errorf("expected DOM visible=false, got %v", v)
	}
}

func TestProxyWriteIDFails(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// Writing to $.id should be silently ignored (read-only).
	// The script should not crash; the value should remain unchanged.
	err := rt.execScript("child1", "test", `
		$.id = "hacked";
		if ($.id !== "child1") {
			throw new Error("id should not have changed, got " + $.id);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyWriteTypeFails(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		$.type = "button";
		if ($.type !== "text") {
			throw new Error("type should not have changed, got " + $.type);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyDirtySetPopulated(t *testing.T) {
	rt := newTestRuntimeWithTree(t)

	// Clear any existing dirty state.
	rt.mu.Lock()
	rt.dirty = make(map[string]bool)
	rt.mu.Unlock()

	err := rt.execScript("child1", "test", `$.text = "changed"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	rt.mu.Lock()
	isDirty := rt.dirty["child1"]
	rt.mu.Unlock()

	if !isDirty {
		t.Error("expected child1 to be in dirty set after write")
	}
}

func TestProxyReadChildren(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("root", "test", `
		var kids = $.children;
		if (kids.length !== 2) {
			throw new Error("expected 2 children, got " + kids.length);
		}
		if (kids[0].id !== "child1") {
			throw new Error("expected first child id='child1', got " + kids[0].id);
		}
		if (kids[1].id !== "child2") {
			throw new Error("expected second child id='child2', got " + kids[1].id);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyWriteToChildrenFails(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	// Children is read-only; assigning should be silently ignored.
	err := rt.execScript("root", "test", `
		$.children = [];
		if ($.children.length !== 2) {
			throw new Error("children should not have changed, got " + $.children.length);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyReadProps(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `
		if ($.props.text !== "hello") {
			throw new Error("expected props.text='hello', got " + $.props.text);
		}
	`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyWriteCustomProp(t *testing.T) {
	rt := newTestRuntimeWithTree(t)
	err := rt.execScript("child1", "test", `$.customProp = "custom_value"`, nil)
	if err != nil {
		t.Fatal(err)
	}

	node := rt.tree.Find("child1")
	v, ok := node.GetProp("customProp")
	if !ok || v != "custom_value" {
		t.Errorf("expected DOM customProp='custom_value', got %v", v)
	}
}

func TestProxyRows(t *testing.T) {
	root, err := dom.NewNode("root", dom.TypeContainer)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := dom.NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := dom.NewNode("tbl", dom.TypeTable)
	if err != nil {
		t.Fatal(err)
	}
	rows := []any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
	}
	tbl.SetProp("rows", rows)
	if err := tree.Insert("root", tbl, ""); err != nil {
		t.Fatal(err)
	}

	events := dom.NewEventQueue()
	rt := New(tree, events)

	err = rt.execScript("tbl", "test", `
		if ($.rows.length !== 2) {
			throw new Error("expected 2 rows, got " + $.rows.length);
		}
	`, nil)
	if err != nil {
		var se *Error
		if errors.As(err, &se) {
			t.Fatalf("script error: %s", se.Message)
		}
		t.Fatal(err)
	}
}
