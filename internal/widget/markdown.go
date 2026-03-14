package widget

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// MarkdownWidget renders markdown content via glamour.
type MarkdownWidget struct{}

// Init implements Widget.
func (w *MarkdownWidget) Init(_ *dom.Node) {}

// Update implements Widget.
func (w *MarkdownWidget) Update(_ tea.Msg, _ *dom.Node) UpdateResult {
	return UpdateResult{}
}

// Layout implements Widget.
func (w *MarkdownWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// View implements Widget.
func (w *MarkdownWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	content := PropString(node, "content", "")
	if content == "" {
		return "(empty)"
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(markdownTheme(node)),
		glamour.WithWordWrap(ctx.Width),
	)
	if err != nil {
		return content
	}
	defer func() {
		_ = renderer.Close()
	}()

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(rendered, "\n")
}

func markdownTheme(node *dom.Node) string {
	theme := PropString(node, "theme", "ascii")
	switch theme {
	case "ascii", "dark", "light", "notty":
		return theme
	default:
		return "ascii"
	}
}
