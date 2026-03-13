package widget

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// DiffWidget implements a diff viewer with unified and split modes.
type DiffWidget struct {
	mode         string // "unified" or "split"
	currentHunk  int
	scrollOffset int
}

type diffHunk struct {
	OldStart int
	NewStart int
	Lines    []diffLine
}

type diffLine struct {
	Type    string // "add", "remove", "context"
	Content string
	OldNum  int
	NewNum  int
}

// Init implements Widget.
func (w *DiffWidget) Init(node *dom.Node) {
	w.mode = PropString(node, "mode", "unified")
}

// Layout implements Widget.
func (w *DiffWidget) Layout(_ *dom.Node, _ ViewContext) []ChildConstraint {
	return nil
}

func (w *DiffWidget) parseHunks(node *dom.Node) []diffHunk {
	raw := PropMapSlice(node, "hunks")
	hunks := make([]diffHunk, 0, len(raw))
	for _, m := range raw {
		h := diffHunk{}
		if v, ok := m["old_start"]; ok {
			h.OldStart = toInt(v)
		}
		if v, ok := m["new_start"]; ok {
			h.NewStart = toInt(v)
		}
		if linesRaw, ok := m["lines"].([]any); ok {
			for _, lr := range linesRaw {
				if lm, ok := lr.(map[string]any); ok {
					dl := diffLine{
						Type:    stringFromMap(lm, "type"),
						Content: stringFromMap(lm, "content"),
					}
					if v, ok := lm["old_num"]; ok {
						dl.OldNum = toInt(v)
					}
					if v, ok := lm["new_num"]; ok {
						dl.NewNum = toInt(v)
					}
					h.Lines = append(h.Lines, dl)
				}
			}
		}
		hunks = append(hunks, h)
	}
	return hunks
}

// Update implements Widget.
func (w *DiffWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	hunks := w.parseHunks(node)

	switch keyMsg.Type {
	case tea.KeyDown:
		w.scrollOffset++
		return UpdateResult{Consumed: true}

	case tea.KeyUp:
		if w.scrollOffset > 0 {
			w.scrollOffset--
		}
		return UpdateResult{Consumed: true}

	case tea.KeyEnter:
		// Emit select_line for current position.
		lines := w.allLines(hunks)
		lineIdx := w.scrollOffset
		if lineIdx < 0 {
			lineIdx = 0
		}
		if lineIdx >= len(lines) {
			lineIdx = len(lines) - 1
		}
		var data map[string]any
		if lineIdx >= 0 && lineIdx < len(lines) {
			l := lines[lineIdx]
			data = map[string]any{
				"type":     l.Type,
				"line_num": l.NewNum,
				"content":  l.Content,
			}
		} else {
			data = map[string]any{}
		}
		return UpdateResult{
			Consumed: true,
			Events: []Event{
				{Type: "select_line", NodeID: node.ID, Data: data},
			},
		}

	case tea.KeyRunes:
		if len(keyMsg.Runes) == 1 {
			switch keyMsg.Runes[0] {
			case 'd':
				if w.mode == "unified" {
					w.mode = "split"
				} else {
					w.mode = "unified"
				}
				return UpdateResult{Consumed: true}

			case 'n':
				if w.currentHunk < len(hunks)-1 {
					w.currentHunk++
				}
				return UpdateResult{
					Consumed: true,
					Events: []Event{
						{Type: "hunk_navigate", NodeID: node.ID, Data: map[string]any{"hunk_index": w.currentHunk}},
					},
				}

			case 'p':
				if w.currentHunk > 0 {
					w.currentHunk--
				}
				return UpdateResult{
					Consumed: true,
					Events: []Event{
						{Type: "hunk_navigate", NodeID: node.ID, Data: map[string]any{"hunk_index": w.currentHunk}},
					},
				}
			}
		}
	}

	return UpdateResult{}
}

func (w *DiffWidget) allLines(hunks []diffHunk) []diffLine {
	var all []diffLine
	for _, h := range hunks {
		all = append(all, h.Lines...)
	}
	return all
}

// View implements Widget.
func (w *DiffWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	hunks := w.parseHunks(node)
	if len(hunks) == 0 {
		style := lipgloss.NewStyle().Width(ctx.Width).Align(lipgloss.Center)
		if ctx.Theme != nil {
			style = mergeStyles(style, ctx.Theme.Resolve("muted"))
		}
		return style.Render("(empty)")
	}

	fileName := PropString(node, "file_name", "")

	var lines []string

	// File header.
	if fileName != "" {
		headerStyle := lipgloss.NewStyle().Bold(true)
		sep := strings.Repeat("─", ctx.Width-len(fileName)-4)
		lines = append(lines, headerStyle.Render(fmt.Sprintf("── %s %s", fileName, sep)))
	}

	// Render each hunk.
	for _, h := range hunks {
		// Hunk header.
		hunkHeader := fmt.Sprintf("@@ -%d +%d @@", h.OldStart, h.NewStart)
		if ctx.Theme != nil {
			hunkHeader = ctx.Theme.Resolve("info").Render(hunkHeader)
		}
		lines = append(lines, hunkHeader)

		for _, l := range h.Lines {
			line := w.renderDiffLine(l, ctx)
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func (w *DiffWidget) renderDiffLine(l diffLine, ctx ViewContext) string {
	gutterWidth := 6 // "  42 " format

	var oldGutter, newGutter string
	if l.OldNum > 0 {
		oldGutter = fmt.Sprintf("%4d", l.OldNum)
	} else {
		oldGutter = "    "
	}
	if l.NewNum > 0 {
		newGutter = fmt.Sprintf("%4d", l.NewNum)
	} else {
		newGutter = "    "
	}

	gutter := oldGutter + " " + newGutter + " │"
	_ = gutterWidth

	var prefix string
	var style lipgloss.Style
	switch l.Type {
	case "add":
		prefix = "+"
		if ctx.Theme != nil {
			style = ctx.Theme.Resolve("add")
		}
	case "remove":
		prefix = "-"
		if ctx.Theme != nil {
			style = ctx.Theme.Resolve("remove")
		}
	default:
		prefix = " "
		style = lipgloss.NewStyle()
	}

	if ctx.Theme != nil {
		gutterStyle := ctx.Theme.Resolve("line-number")
		gutter = gutterStyle.Render(gutter)
	}

	content := style.Render(prefix + l.Content)
	return gutter + content
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}
