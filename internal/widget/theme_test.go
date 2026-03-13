package widget

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()
	if theme == nil {
		t.Fatal("DefaultTheme returned nil")
	}
}

func TestTheme_Resolve_SingleToken(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		token string
		check func(lipgloss.Style) bool
		desc  string
	}{
		{"bold", func(s lipgloss.Style) bool { return s.GetBold() }, "should be bold"},
		{"dim", func(s lipgloss.Style) bool { return s.GetFaint() }, "should be faint"},
		{"italic", func(s lipgloss.Style) bool { return s.GetItalic() }, "should be italic"},
		{"underline", func(s lipgloss.Style) bool { return s.GetUnderline() }, "should be underlined"},
		{"strikethrough", func(s lipgloss.Style) bool { return s.GetStrikethrough() }, "should be strikethrough"},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			style := theme.Resolve(tt.token)
			if !tt.check(style) {
				t.Errorf("token %q: %s", tt.token, tt.desc)
			}
		})
	}
}

func TestTheme_Resolve_ColorTokens(t *testing.T) {
	theme := DefaultTheme()

	// Color tokens should produce styles with a foreground color set.
	tests := []struct {
		token    string
		wantColor lipgloss.TerminalColor
	}{
		{"danger", lipgloss.Color("9")},
		{"success", lipgloss.Color("10")},
		{"warning", lipgloss.Color("11")},
		{"info", lipgloss.Color("12")},
		{"muted", lipgloss.Color("8")},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			style := theme.Resolve(tt.token)
			got := style.GetForeground()
			if got != tt.wantColor {
				t.Errorf("token %q: foreground = %v, want %v", tt.token, got, tt.wantColor)
			}
		})
	}
}

func TestTheme_Resolve_MultipleTokens(t *testing.T) {
	theme := DefaultTheme()

	style := theme.Resolve("bold italic")
	if !style.GetBold() {
		t.Error("expected bold")
	}
	if !style.GetItalic() {
		t.Error("expected italic")
	}
}

func TestTheme_Resolve_EmptyString(t *testing.T) {
	theme := DefaultTheme()
	style := theme.Resolve("")
	// Should return a zero-value style without panicking.
	rendered := style.Render("x")
	if rendered == "" {
		t.Error("empty token should still render text")
	}
}

func TestTheme_Resolve_UnknownToken(t *testing.T) {
	theme := DefaultTheme()
	// Unknown tokens are silently ignored — result is a zero-value style.
	style := theme.Resolve("nonexistent")
	if style.GetBold() || style.GetItalic() || style.GetFaint() {
		t.Error("unknown token should produce unstyled output")
	}
	if style.GetForeground() != (lipgloss.NoColor{}) {
		t.Error("unknown token should have no foreground color")
	}
}

func TestTheme_Resolve_MixedKnownUnknown(t *testing.T) {
	theme := DefaultTheme()
	style := theme.Resolve("bold nonexistent italic")
	if !style.GetBold() {
		t.Error("expected bold")
	}
	if !style.GetItalic() {
		t.Error("expected italic")
	}
}
