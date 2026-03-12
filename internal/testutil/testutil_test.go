package testutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joncooper/imagine-tui/internal/testutil"
)

func TestGoldenFileRoundTrip(t *testing.T) {
	// Create a golden file, then verify it matches.
	name := "testutil/roundtrip.golden"
	content := []byte("hello golden\n")

	// First, set GOLDEN_UPDATE to create the file.
	t.Setenv("GOLDEN_UPDATE", "1")
	testutil.GoldenFile(t, name, content)

	// Now unset GOLDEN_UPDATE and verify it matches.
	t.Setenv("GOLDEN_UPDATE", "")
	testutil.GoldenFile(t, name, content)

	// Clean up the temporary golden file.
	t.Cleanup(func() {
		dir, _ := os.Getwd()
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				_ = os.Remove(filepath.Join(dir, testutil.GoldenDir, name))
				_ = os.Remove(filepath.Join(dir, testutil.GoldenDir, "testutil"))
				break
			}
			dir = filepath.Dir(dir)
		}
	})
}

func TestAssertNodeProps(t *testing.T) {
	got := map[string]any{
		"text":  "hello",
		"width": 42,
		"extra": "ignored",
	}
	expected := map[string]any{
		"text":  "hello",
		"width": 42,
	}
	// Should not fail — expected is a subset of got.
	testutil.AssertNodeProps(t, got, expected)
}
