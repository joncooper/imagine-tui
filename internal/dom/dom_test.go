package dom_test

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
)

func TestBubblesImport(t *testing.T) {
	// Verify bubbles dependency is available.
	ti := textinput.New()
	if ti.Placeholder != "" {
		t.Fatal("expected empty placeholder")
	}
	t.Log("bubbles import OK")
}
