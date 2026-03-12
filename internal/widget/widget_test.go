package widget_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestLipGlossImport(t *testing.T) {
	// Verify lipgloss dependency is available.
	s := lipgloss.NewStyle().Bold(true)
	result := s.Render("hello")
	if result == "" {
		t.Fatal("lipgloss render returned empty string")
	}
	t.Log("lipgloss import OK")
}
