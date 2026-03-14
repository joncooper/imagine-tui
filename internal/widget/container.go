package widget

import (
	"strconv"
	"strings"

	bviewport "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// ContainerWidget implements flex-like layout with vertical/horizontal direction.
type ContainerWidget struct {
	vp      bviewport.Model
	vpReady bool
}

// Init implements Widget.
func (w *ContainerWidget) Init(_ *dom.Node) {
	w.vp = bviewport.New(0, 0)
}

// Update implements Widget.
func (w *ContainerWidget) Update(msg tea.Msg, node *dom.Node) UpdateResult {
	if !w.vpReady || PropString(node, "overflow", "") != "scroll" {
		return UpdateResult{}
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return UpdateResult{}
	}

	switch keyMsg.Type {
	case tea.KeyDown:
		w.vp.ScrollDown(1)
	case tea.KeyUp:
		w.vp.ScrollUp(1)
	case tea.KeyPgDown:
		w.vp.PageDown()
	case tea.KeyPgUp:
		w.vp.PageUp()
	case tea.KeyHome:
		w.vp.GotoTop()
	case tea.KeyEnd:
		w.vp.GotoBottom()
	default:
		return UpdateResult{}
	}

	return UpdateResult{Consumed: true}
}

// Layout implements Widget.
func (w *ContainerWidget) Layout(node *dom.Node, ctx ViewContext) []ChildConstraint {
	if len(node.Children) == 0 {
		return nil
	}

	direction := PropString(node, "direction", "vertical")
	gap := PropInt(node, "gap", 0)
	padding := PropInt(node, "padding", 0)

	available := ctx.Width - 2*padding
	available -= borderWidth(PropString(node, "border", "none"))

	availableHeight := ctx.Height - 2*padding
	availableHeight -= borderHeight(PropString(node, "border", "none"))

	if direction == "vertical" {
		return w.layoutVertical(node, available, availableHeight)
	}
	return w.layoutHorizontal(node, available, availableHeight, gap)
}

func (w *ContainerWidget) layoutVertical(node *dom.Node, available, height int) []ChildConstraint {
	constraints := make([]ChildConstraint, len(node.Children))
	for i := range constraints {
		constraints[i].Width = available
		constraints[i].Height = height
	}
	return constraints
}

func (w *ContainerWidget) layoutHorizontal(node *dom.Node, available, height, gap int) []ChildConstraint {
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
			switch {
			case val == "fill":
				specs[i] = sizing{kind: "fill"}
			case strings.HasSuffix(val, "%"):
				pct, err := strconv.Atoi(strings.TrimSuffix(val, "%"))
				if err == nil {
					specs[i] = sizing{kind: "percent", value: pct}
				} else {
					specs[i] = sizing{kind: "auto"}
				}
			default:
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

	// All horizontal children share the same height.
	for i := range constraints {
		constraints[i].Height = height
	}

	return constraints
}

// View implements Widget.
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

	height, hasHeight := PropSize(node, "height", ctx.Height)
	maxHeight, hasMaxHeight := PropSize(node, "max_height", ctx.Height)
	if hasHeight {
		style = style.Height(height)
	}
	if hasMaxHeight && !hasHeight {
		style = style.MaxHeight(maxHeight)
	}

	if PropString(node, "overflow", "") == "scroll" {
		if inner, ok := w.viewportContent(node, composed, ctx.Width, height, hasHeight, maxHeight, hasMaxHeight); ok {
			composed = inner
		}
	} else {
		w.vpReady = false
	}

	if composed == "" {
		return style.Render("")
	}
	return style.Render(composed)
}

func (w *ContainerWidget) viewportContent(node *dom.Node, composed string, width, height int, hasHeight bool, maxHeight int, hasMaxHeight bool) (string, bool) {
	padding := PropInt(node, "padding", 0)
	borderName := PropString(node, "border", "none")

	var outerHeight int
	switch {
	case hasHeight:
		outerHeight = height
	case hasMaxHeight:
		outerHeight = maxHeight
	default:
		w.vpReady = false
		return "", false
	}

	innerWidth := width - 2*padding - borderWidth(borderName)
	innerHeight := outerHeight - 2*padding - borderHeight(borderName)
	if innerWidth <= 0 || innerHeight <= 0 {
		w.vpReady = false
		return "", false
	}

	attachViewport := hasHeight
	if !attachViewport && hasMaxHeight {
		attachViewport = lineCount(composed) > innerHeight
	}
	if !attachViewport {
		w.vpReady = false
		return "", false
	}

	w.vp.Width = innerWidth
	w.vp.Height = innerHeight
	w.vp.SetContent(composed)
	clampViewport(&w.vp)
	w.vpReady = true
	return w.vp.View(), true
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func clampViewport(vp *bviewport.Model) {
	maxOffset := vp.TotalLineCount() - vp.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if vp.YOffset > maxOffset {
		vp.SetYOffset(maxOffset)
	}
	if vp.YOffset < 0 {
		vp.SetYOffset(0)
	}
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

// borderHeight returns the total vertical height consumed by a border style.
func borderHeight(name string) int {
	b := resolveBorder(name)
	if b == nil {
		return 0
	}
	h := 0
	if b.Top != "" {
		h++
	}
	if b.Bottom != "" {
		h++
	}
	return h
}
