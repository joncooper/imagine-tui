package widget

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme maps style token names to Lip Gloss styles.
type Theme struct {
	tokens map[string]lipgloss.Style
}

// DefaultTheme returns the built-in theme with standard tokens.
func DefaultTheme() *Theme {
	return &Theme{
		tokens: map[string]lipgloss.Style{
			// Text formatting.
			"bold":          lipgloss.NewStyle().Bold(true),
			"dim":           lipgloss.NewStyle().Faint(true),
			"italic":        lipgloss.NewStyle().Italic(true),
			"underline":     lipgloss.NewStyle().Underline(true),
			"strikethrough": lipgloss.NewStyle().Strikethrough(true),

			// Semantic colors.
			"danger":  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			"success": lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			"warning": lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			"info":    lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
			"muted":   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

			// Widget state tokens (used by widgets internally).
			"focused":  lipgloss.NewStyle().BorderForeground(lipgloss.Color("12")),
			"disabled": lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("8")),
			"selected": lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")),

			// Structural tokens.
			"header":      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
			"line-number": lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
			"add":         lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			"remove":      lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		},
	}
}

// Resolve splits a space-separated token string and merges the matching styles
// left-to-right. Unknown tokens are silently ignored.
func (t *Theme) Resolve(tokenStr string) lipgloss.Style {
	style := lipgloss.NewStyle()
	if tokenStr == "" {
		return style
	}
	for _, token := range strings.Fields(tokenStr) {
		if s, ok := t.tokens[token]; ok {
			style = mergeStyles(style, s)
		}
	}
	return style
}

// mergeStyles copies non-zero style attributes from src onto dst.
func mergeStyles(dst, src lipgloss.Style) lipgloss.Style {
	if src.GetBold() {
		dst = dst.Bold(true)
	}
	if src.GetFaint() {
		dst = dst.Faint(true)
	}
	if src.GetItalic() {
		dst = dst.Italic(true)
	}
	if src.GetUnderline() {
		dst = dst.Underline(true)
	}
	if src.GetStrikethrough() {
		dst = dst.Strikethrough(true)
	}
	if fg := src.GetForeground(); fg != (lipgloss.NoColor{}) {
		dst = dst.Foreground(fg)
	}
	if bg := src.GetBackground(); bg != (lipgloss.NoColor{}) {
		dst = dst.Background(bg)
	}
	return dst
}
