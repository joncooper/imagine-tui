package render_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBubbleTeaImport(t *testing.T) {
	// Verify bubbletea dependency is available and the core type is usable.
	var _ tea.Model
	t.Log("bubbletea import OK")
}
