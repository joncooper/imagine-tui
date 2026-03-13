package widget

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// stubWidget is a minimal Widget implementation for testing infrastructure.
type stubWidget struct {
	typeName    string
	initialized bool
	lastMsg     tea.Msg
	viewCalls   int
}

func (s *stubWidget) Init(node *dom.Node) {
	s.initialized = true
}

func (s *stubWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	s.lastMsg = msg
	return UpdateResult{}
}

func (s *stubWidget) View(node *dom.Node, children []RenderedChild, ctx ViewContext) string {
	s.viewCalls++
	text, _ := node.Props["text"].(string)
	if text == "" {
		text = node.ID
	}
	return text
}

func (s *stubWidget) Layout(node *dom.Node, ctx ViewContext) []ChildConstraint {
	return nil
}
