package widget

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// CodeWidget implements a code block with line numbers and line highlighting.
type CodeWidget struct {
	scrollOffset int
}

// Init implements Widget.
func (w *CodeWidget) Init(_ *dom.Node) {}

// Layout implements Widget.
func (w *CodeWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

// Update implements Widget.
func (w *CodeWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	content := PropString(node, "content", "")
	lineCount := len(strings.Split(content, "\n"))

	switch keyMsg.Type {
	case tea.KeyDown:
		if w.scrollOffset < lineCount-1 {
			w.scrollOffset++
		}
		return UpdateResult{Consumed: true}

	case tea.KeyUp:
		if w.scrollOffset > 0 {
			w.scrollOffset--
		}
		return UpdateResult{Consumed: true}

	default:
		return UpdateResult{}
	}
}

// View implements Widget.
func (w *CodeWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	content := PropString(node, "content", "")
	if content == "" {
		style := lipgloss.NewStyle().Width(ctx.Width).Align(lipgloss.Center)
		if ctx.Theme != nil {
			style = mergeStyles(style, ctx.Theme.Resolve("muted"))
		}
		return style.Render("(empty)")
	}

	showLineNums := PropBool(node, "line_numbers", true)
	startLine := PropInt(node, "start_line", 1)
	highlightSet := w.parseHighlightLines(node)

	lines := strings.Split(content, "\n")
	// Remove trailing empty line from trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	totalLines := len(lines)
	gutterWidth := len(fmt.Sprintf("%d", startLine+totalLines-1))

	var rendered []string
	for i, line := range lines {
		lineNum := startLine + i
		isHighlighted := highlightSet[lineNum]

		var lineStr string
		if showLineNums {
			numStr := fmt.Sprintf("%*d", gutterWidth, lineNum)
			if ctx.Theme != nil {
				numStr = ctx.Theme.Resolve("line-number").Render(numStr)
			}
			lineStr = numStr + " │ " + line
		} else {
			lineStr = line
		}

		if isHighlighted {
			hlStyle := lipgloss.NewStyle().Background(lipgloss.Color("236"))
			lineStr = hlStyle.Render(lineStr)
		}

		rendered = append(rendered, lineStr)
	}

	return strings.Join(rendered, "\n")
}

// parseHighlightLines builds a set of line numbers to highlight.
// Supports individual ints and range strings like "3-7".
func (w *CodeWidget) parseHighlightLines(node *dom.Node) map[int]bool {
	v, ok := node.GetProp("highlight_lines")
	if !ok {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}

	result := make(map[int]bool)
	for _, item := range items {
		switch val := item.(type) {
		case int:
			result[val] = true
		case float64:
			result[int(val)] = true
		case string:
			// Parse range "3-7".
			parts := strings.SplitN(val, "-", 2)
			if len(parts) == 2 {
				start, err1 := strconv.Atoi(parts[0])
				end, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					for n := start; n <= end; n++ {
						result[n] = true
					}
				}
			} else if n, err := strconv.Atoi(val); err == nil {
				result[n] = true
			}
		}
	}
	return result
}
