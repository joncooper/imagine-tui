// Package testutil provides test helpers for the imagine-tui project.
//
// Helpers include DOM construction shortcuts, assertion utilities, and golden
// file infrastructure for widget rendering tests.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// GoldenDir is the path to the golden file directory relative to the repo root.
const GoldenDir = "testdata/golden"

// GoldenFile compares got against the golden file at name. If the GOLDEN_UPDATE
// environment variable is set to "1", the golden file is overwritten with got
// instead of compared.
//
// The name should be a slash-separated path relative to testdata/golden/
// (e.g., "widget/text_plain.golden").
func GoldenFile(t *testing.T, name string, got []byte) {
	t.Helper()

	path := goldenPath(t, name)

	if os.Getenv("GOLDEN_UPDATE") == "1" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create golden dir %s: %v", dir, err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden file %s: %v", path, err)
		}
		t.Logf("updated golden file %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with GOLDEN_UPDATE=1 to create)", path, err)
	}

	if string(got) != string(want) {
		t.Errorf("golden file mismatch %s\n--- want ---\n%s\n--- got ---\n%s",
			name, string(want), string(got))
	}
}

// goldenPath resolves the golden file path by walking up from the test's
// working directory to find testdata/golden/.
func goldenPath(t *testing.T, name string) string {
	t.Helper()

	// Tests run from their package directory. Walk up to find the repo root
	// by looking for go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, GoldenDir, name)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root (go.mod) from working directory")
		}
		dir = parent
	}
}

// AssertNodeProps checks that a map contains all expected key-value pairs.
// Extra keys in got are ignored. This is useful for checking a subset of node
// properties without asserting the entire map.
func AssertNodeProps(t *testing.T, got map[string]any, expected map[string]any) {
	t.Helper()
	for k, wantV := range expected {
		gotV, ok := got[k]
		if !ok {
			t.Errorf("missing prop %q: expected %v", k, wantV)
			continue
		}
		// Use %v comparison for simplicity; callers can use more specific
		// checks for complex types.
		if gotStr, wantStr := stringify(gotV), stringify(wantV); gotStr != wantStr {
			t.Errorf("prop %q: got %s, want %s", k, gotStr, wantStr)
		}
	}
}

func stringify(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
