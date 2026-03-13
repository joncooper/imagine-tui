package widget

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// ContainerWidget implements flex-like layout with vertical/horizontal direction.
type ContainerWidget struct{}

func (w *ContainerWidget) Init(_ *dom.Node) {}

func (w *ContainerWidget) Update(_ tea.Msg, _ *dom.Node) UpdateResult {
	return UpdateResult{}
}

func (w *ContainerWidget) Layout(node *dom.Node, ctx ViewContext) []ChildConstraint {
	if len(node.Children) == 0 {
		return nil
	}

	direction := PropString(node, "direction", "vertical")
	gap := PropInt(node, "gap", 0)
	padding := PropInt(node, "padding", 0)

	available := ctx.Width - 2*padding
	available -= borderWidth(PropString(node, "border", "none"))

	if direction == "vertical" {
		return w.layoutVertical(node, available)
	}
	return w.layoutHorizontal(node, available, gap)
}

func (w *ContainerWidget) layoutVertical(node *dom.Node, available int) []ChildConstraint {
	constraints := make([]ChildConstraint, len(node.Children))
	for i := range constraints {
		constraints[i].Width = available
	}
	return constraints
}

func (w *ContainerWidget) layoutHorizontal(node *dom.Node, available, gap int) []ChildConstraint {
	n := len(node.Children)
	totalGap := gap * (n - 1)
	usable := available - totalGap
	if usable < 0 {
		usable = 0
	}

	constraints := make([]ChildConstraint, n)

	type sizing struct {
		kind  string // "fixed", "percent", "fill", "auto"
		value int    // fixed width or percentage
	}

	specs := make([]sizing, n)
	for i, child := range node.Children {
		v, ok := child.GetProp("width")
		if !ok {
			specs[i] = sizing{kind: "auto"}
			continue
		}
		switch val := v.(type) {
		case int:
			specs[i] = sizing{kind: "fixed", value: val}
		case float64:
			specs[i] = sizing{kind: "fixed", value: int(val)}
		case string:
			if val == "fill" {
				specs[i] = sizing{kind: "fill"}
			} else if strings.HasSuffix(val, "%") {
				pct, err := strconv.Atoi(strings.TrimSuffix(val, "%"))
				if err == nil {
					specs[i] = sizing{kind: "percent", value: pct}
				} else {
					specs[i] = sizing{kind: "auto"}
				}
			} else {
				specs[i] = sizing{kind: "auto"}
			}
		default:
			specs[i] = sizing{kind: "auto"}
		}
	}

	remaining := usable

	// Pass 1: allocate fixed-width children.
	for i, s := range specs {
		if s.kind == "fixed" {
			constraints[i].Width = s.value
			remaining -= s.value
		}
	}

	// Pass 2: allocate percentage children (percentage of usable, not remaining).
	for i, s := range specs {
		if s.kind == "percent" {
			w := usable * s.value / 100
			constraints[i].Width = w
			remaining -= w
		}
	}

	// Pass 3: count fill and auto children, split remaining evenly.
	var flexCount int
	for _, s := range specs {
		if s.kind == "fill" || s.kind == "auto" {
			flexCount++
		}
	}
	if flexCount > 0 && remaining > 0 {
		each := remaining / flexCount
		for i, s := range specs {
			if s.kind == "fill" || s.kind == "auto" {
				constraints[i].Width = each
			}
		}
	}

	return constraints
}

func (w *ContainerWidget) View(node *dom.Node, children []RenderedChild, ctx ViewContext) string {
	if ctx.Width <= 0 {
		return ""
	}

	direction := PropString(node, "direction", "vertical")
	gap := PropInt(node, "gap", 0)
	borderName := PropString(node, "border", "none")
	padding := PropInt(node, "padding", 0)

	// Collect child view strings.
	views := make([]string, 0, len(children))
	for _, c := range children {
		if c.View != "" {
			views = append(views, c.View)
		}
	}

	var composed string
	if direction == "horizontal" {
		composed = joinHorizontal(views, gap)
	} else {
		composed = joinVertical(views, gap)
	}

	// Apply border.
	style := lipgloss.NewStyle()
	if border := resolveBorder(borderName); border != nil {
		style = style.Border(*border)
	}
	if padding > 0 {
		style = style.Padding(padding)
	}

	// Apply style tokens.
	if tokenStr := PropString(node, "style", ""); tokenStr != "" && ctx.Theme != nil {
		tokenStyle := ctx.Theme.Resolve(tokenStr)
		style = mergeStyles(style, tokenStyle)
	}

	if composed == "" {
		return style.Render("")
	}
	return style.Render(composed)
}

func joinHorizontal(views []string, gap int) string {
	if len(views) == 0 {
		return ""
	}
	if gap > 0 {
		spacer := strings.Repeat(" ", gap)
		spaced := make([]string, 0, len(views)*2-1)
		for i, v := range views {
			if i > 0 {
				spaced = append(spaced, spacer)
			}
			spaced = append(spaced, v)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, spaced...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, views...)
}

func joinVertical(views []string, gap int) string {
	if len(views) == 0 {
		return ""
	}
	if gap > 0 {
		spacer := strings.Repeat("\n", gap)
		spaced := make([]string, 0, len(views)*2-1)
		for i, v := range views {
			if i > 0 {
				spaced = append(spaced, spacer)
			}
			spaced = append(spaced, v)
		}
		return lipgloss.JoinVertical(lipgloss.Left, spaced...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

func resolveBorder(name string) *lipgloss.Border {
	switch name {
	case "rounded":
		b := lipgloss.RoundedBorder()
		return &b
	case "thick":
		b := lipgloss.ThickBorder()
		return &b
	case "double":
		b := lipgloss.DoubleBorder()
		return &b
	case "hidden":
		b := lipgloss.HiddenBorder()
		return &b
	case "normal":
		b := lipgloss.NormalBorder()
		return &b
	default:
		return nil
	}
}

// borderWidth returns the total horizontal width consumed by a border style.
func borderWidth(name string) int {
	b := resolveBorder(name)
	if b == nil {
		return 0
	}
	left := len([]rune(b.Left))
	right := len([]rune(b.Right))
	return left + right
}
