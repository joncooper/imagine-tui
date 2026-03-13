package widget

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func testNode(t *testing.T, props map[string]any) *dom.Node {
	t.Helper()
	n, err := dom.NewNode("test", dom.TypeText)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range props {
		n.SetProp(k, v)
	}
	return n
}

func TestPropString(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]any
		key      string
		fallback string
		want     string
	}{
		{"present", map[string]any{"text": "hello"}, "text", "", "hello"},
		{"missing", map[string]any{}, "text", "default", "default"},
		{"wrong type", map[string]any{"text": 42}, "text", "default", "default"},
		{"nil value", map[string]any{"text": nil}, "text", "default", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode(t, tt.props)
			got := PropString(n, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPropInt(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]any
		key      string
		fallback int
		want     int
	}{
		{"int", map[string]any{"n": 42}, "n", 0, 42},
		{"float64", map[string]any{"n": float64(42)}, "n", 0, 42},
		{"missing", map[string]any{}, "n", 5, 5},
		{"wrong type", map[string]any{"n": "not a number"}, "n", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode(t, tt.props)
			got := PropInt(n, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPropBool(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]any
		key      string
		fallback bool
		want     bool
	}{
		{"true", map[string]any{"v": true}, "v", false, true},
		{"false", map[string]any{"v": false}, "v", true, false},
		{"missing", map[string]any{}, "v", true, true},
		{"wrong type", map[string]any{"v": "yes"}, "v", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode(t, tt.props)
			got := PropBool(n, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPropStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]any
		key   string
		want  int // expected length
	}{
		{"present", map[string]any{"items": []any{"a", "b", "c"}}, "items", 3},
		{"missing", map[string]any{}, "items", 0},
		{"wrong type", map[string]any{"items": 42}, "items", 0},
		{"mixed types in slice", map[string]any{"items": []any{"a", 42, "c"}}, "items", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode(t, tt.props)
			got := PropStringSlice(n, tt.key)
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestPropMapSlice(t *testing.T) {
	rows := []any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
	}
	tests := []struct {
		name  string
		props map[string]any
		key   string
		want  int
	}{
		{"present", map[string]any{"rows": rows}, "rows", 2},
		{"missing", map[string]any{}, "rows", 0},
		{"wrong type", map[string]any{"rows": "not a slice"}, "rows", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode(t, tt.props)
			got := PropMapSlice(n, tt.key)
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}
