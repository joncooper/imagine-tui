# Milestone 4: Widget Library — Design Document

## Overview

M4 builds the v1 widget renderers: 11 widget types, each converting a `dom.Node`
with typed props into styled terminal output via Lip Gloss. The widgets are the
visual layer; the DOM is the semantic layer. Widgets read props, manage local
display state (scroll offset, cursor position), and produce events. They never
talk to MCP or goja directly.

### What M4 is NOT

- Not BubbleTea integration (M5 wires widgets into the tea.Model)
- Not script execution (M3 mutates DOM props; widgets just read them)
- Not MCP protocol handling (M2 applies patches; widgets render the result)

M4 can be built and fully tested without M3 because widgets only depend on
`dom.Node` props. Whether a prop was set by a `patch` tool call or by a goja
script is invisible to the widget.

---

## Core abstractions

### Widget interface

```go
// Widget is the contract every widget type implements.
// One instance is created per DOM node and persists for the node's lifetime.
type Widget interface {
    // Init is called once when the widget instance is created for a node.
    // The widget reads initial props and sets up internal state.
    Init(node *dom.Node)

    // Update handles a terminal input event (keypress, mouse).
    // It mutates widget-internal state and returns any DOM events produced.
    // The node is passed so the widget can write back state (e.g., input value).
    // Returns whether the event was consumed.
    Update(msg tea.Msg, node *dom.Node) UpdateResult

    // View renders the widget to a string given current node props,
    // pre-rendered child views (empty for leaf widgets), and a context
    // with allocated dimensions and focus state.
    View(node *dom.Node, children []RenderedChild, ctx ViewContext) string

    // Layout computes the dimensions to allocate to each child node.
    // Only meaningful for container types; leaf widgets return nil.
    Layout(node *dom.Node, ctx ViewContext) []ChildConstraint
}
```

### Supporting types

```go
type UpdateResult struct {
    Consumed bool           // true if the event was handled (stop bubbling)
    Events   []Event  // events produced (routed by the runtime)
}

type Event struct {
    Type   string         // "change", "submit", "click", "select", etc.
    NodeID string         // source node ID
    Data   map[string]any // event payload
}

type ViewContext struct {
    Width   int    // allocated width in columns
    Height  int    // allocated height in rows (0 = unconstrained)
    Focused bool   // whether this node currently has focus
    Theme   *Theme // style token resolver
}

type ChildConstraint struct {
    Width  int // allocated width for this child (0 = unconstrained)
    Height int // allocated height for this child (0 = unconstrained)
}

type RenderedChild struct {
    NodeID string // which child this is
    View   string // pre-rendered output
    Width  int    // actual rendered width
    Height int    // actual rendered height
}
```

### Why this shape

**Post-order rendering with top-down layout.** The rendering pipeline does two
passes over the DOM tree:

1. **Layout pass (top-down):** Starting from root, each container's `Layout()`
   computes `ChildConstraint` for each child based on the container's own
   allocated size and its layout props (direction, gap, child sizing). These
   constraints flow down the tree.

2. **Render pass (bottom-up):** Leaf widgets render first. Their output strings
   are collected into `RenderedChild` slices and passed up to parent containers,
   which compose them with `lipgloss.JoinVertical` / `JoinHorizontal`.

This two-pass approach is necessary because a horizontal container at width 80
with three children at 25%/25%/fill needs to tell each child its allocated width
before the child can render. But it also needs the rendered child strings to
compose its own output.

**Widgets own display state, not semantic state.** A table widget tracks its
scroll offset and expanded rows internally. The DOM node's `rows` prop is the
data; the widget's scroll offset is how that data is currently viewed. When the
DOM prop changes (via patch or script), the widget sees the new value on the
next `View()` call.

**Input writeback.** Input-like widgets (input, textarea, select) are special:
user interaction changes the node's semantic value. These widgets write back to
`node.Props["value"]` during `Update()` so that:
- Scripts can read `$('myinput').value`
- The `query` tool returns current values
- Event context auto-collection captures sibling state

---

## Widget registry

```go
// Registry maps node types to widget factories.
type Registry struct {
    factories map[dom.NodeType]func() Widget
}

func NewRegistry() *Registry
func (r *Registry) Register(nodeType dom.NodeType, factory func() Widget)
func (r *Registry) Create(nodeType dom.NodeType) (Widget, error)
```

The registry is populated at startup with all v1 widget types. `Create` returns
a new instance (widgets are stateful — each DOM node gets its own). Unknown
node types return an error.

### Widget instance management

```go
// Tree manages the lifecycle of widget instances for a DOM tree.
type Tree struct {
    registry  *Registry
    instances map[string]Widget // keyed by node ID
}

func NewTree(registry *Registry) *Tree
func (wt *Tree) Sync(tree *dom.Tree) error     // create/remove instances to match DOM
func (wt *Tree) Get(nodeID string) Widget       // nil if not found
func (wt *Tree) Render(tree *dom.Tree, width, height int, focusedID string) string
```

`Sync` walks the DOM tree and:
- Creates + `Init()`s widget instances for nodes that don't have one yet
- Removes widget instances for nodes no longer in the tree

`Sync` is called after every DOM mutation (patch, replace, restore). The cost is
a tree walk + map lookups — cheap relative to rendering.

`Render` orchestrates the two-pass pipeline (layout then render) and returns the
composed terminal output string.

---

## Style token system

Widgets need styling. Rather than exposing raw Lip Gloss config over MCP (which
would be verbose and brittle), the API uses a token system. Claude sends
`"style": "bold danger"` in props; the widget resolves that to Lip Gloss styles.

```go
type Theme struct {
    tokens map[string]lipgloss.Style
}

func DefaultTheme() *Theme
func (t *Theme) Resolve(tokenStr string) lipgloss.Style
```

`Resolve` splits the token string on whitespace and merges matching styles
left-to-right. Unknown tokens are ignored (no error — forward-compatible).

### Built-in tokens

**Text formatting:**
`bold`, `dim`, `italic`, `underline`, `strikethrough`

**Semantic colors:**
`danger` (red), `success` (green), `warning` (yellow), `info` (blue), `muted` (gray)

**Widget state:**
`focused` (accent border/fg), `disabled` (dim + muted), `selected` (inverted or accent bg)

**Structural** (applied by widgets internally, not user-facing):
`header` (table column headers), `line-number` (code/diff gutter),
`add` (diff additions), `remove` (diff deletions)

### How widgets use the theme

Widgets resolve the node's `style` prop through the theme for text/color styling.
Structural styling (border type, padding, width) comes from dedicated props, not
tokens. Example for a text widget:

```go
func (w *TextWidget) View(node *dom.Node, _ []RenderedChild, ctx ViewContext) string {
    text, _ := node.Props["text"].(string)
    style := ctx.Theme.Resolve(node.Props["style"])
    style = style.Width(ctx.Width)
    return style.Render(text)
}
```

---

## Shared utilities

Several widgets share behavior. These are internal helpers, not public API.

### Viewport (scrollable content)

Used by: table, list, log, code, diff, textarea

```go
type viewport struct {
    offset     int  // first visible line
    height     int  // visible lines
    totalLines int  // total content lines
    sticky     bool // auto-scroll to bottom (log widget)
}

func (v *viewport) scrollDown(n int)
func (v *viewport) scrollUp(n int)
func (v *viewport) scrollToBottom()
func (v *viewport) isAtBottom() bool
func (v *viewport) visibleRange() (start, end int)
func (v *viewport) clamp()
```

The `sticky` field implements the log widget's "sticky-bottom" behavior:
auto-scroll to bottom unless user scrolled up manually; re-engage when
user scrolls back to bottom.

### Prop helpers

Type-safe prop extraction with defaults:

```go
func propString(node *dom.Node, key string, fallback string) string
func propInt(node *dom.Node, key string, fallback int) int
func propBool(node *dom.Node, key string, fallback bool) bool
func propStringSlice(node *dom.Node, key string) []string
func propMapSlice(node *dom.Node, key string) []map[string]any
```

These handle the `map[string]any` → typed value conversion that every widget
needs. They never panic — invalid types return the fallback.

---

## Per-widget design

### container (M4-2)

The structural backbone. The only widget whose `Layout()` and `View()` both do
real work (other widgets return nil from Layout).

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `direction` | `"vertical"` \| `"horizontal"` | `"vertical"` | Layout axis |
| `gap` | int | 0 | Characters between children |
| `padding` | int or `{top,right,bottom,left}` | 0 | Inner padding |
| `border` | `"none"` \| `"rounded"` \| `"thick"` \| `"double"` \| `"hidden"` | `"none"` | Border style |
| `style` | token string | `""` | Text/color styling for the container itself |

**Child sizing:** Each child node can declare a `width` prop that the container
interprets:

| Value | Meaning |
|-------|---------|
| (absent) | Auto-sized to content |
| integer | Fixed character width |
| `"50%"` | Percentage of available space |
| `"fill"` | Takes remaining space after fixed/percentage children |

Height follows the same scheme via a `height` prop, but only applies in
horizontal layout (in vertical layout, each child takes as much height as it
needs).

**Layout algorithm** (for horizontal direction; vertical is symmetric):

```
available = container_width - padding_left - padding_right - border_width
available -= gap * (num_children - 1)

1. Allocate fixed-width children first (subtract from available)
2. Allocate percentage children (percentage of original available, subtract)
3. Remaining space divided equally among "fill" children
4. Auto-sized children get min(content_width, remaining_space)
```

**Focus cycling:** Tab/Shift-Tab moves focus among focusable descendants. This
is a container default behavior. The container maintains a `focusRing []string`
(ordered list of focusable node IDs, built by walking descendants). Focus state
is not per-container — it's a single `focusedID` managed by the runtime and
passed through ViewContext. The container's Update() handles Tab by advancing
the focus ring and returning the new focused ID in the event.

**Widget state:** `focusRing []string` (cached, rebuilt on Sync)

---

### text (M4-3)

The simplest widget. No internal state, no Update behavior.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `text` | string | `""` | Content to display |
| `style` | token string | `""` | Style tokens |
| `wrap` | bool | true | Word-wrap at allocated width |
| `max_lines` | int | 0 | Truncate with ellipsis after N lines (0 = no limit) |
| `segments` | `[{text, style}]` | nil | Mixed-style inline spans (overrides `text` if present) |

**Segments** support inline style variation. Example: an annotation line with
a severity tag:

```json
{
  "segments": [
    {"text": "[ERROR] ", "style": "bold danger"},
    {"text": "nil pointer dereference in ", "style": ""},
    {"text": "handler.go:42", "style": "underline info"}
  ]
}
```

Each segment is rendered with its own resolved style, then concatenated.

**Rendering:** Apply style tokens, set width, handle word wrap via
`lipgloss.NewStyle().Width(w).MaxHeight(h).Render(text)`. For segments,
render each independently and concatenate before applying outer width.

---

### input (M4-4)

Single-line text input.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `placeholder` | string | `""` | Shown when value is empty |
| `value` | string | `""` | Current text value |
| `pattern` | string (regex) | `""` | Validation pattern |
| `style` | token string | `""` | Base style |

**Events emitted:**

| Event | When | Data |
|-------|------|------|
| `change` | Every keystroke | `{value}` |
| `submit` | Enter pressed | `{value}` |

**Widget state:** cursor position, current value (synced back to `node.Props["value"]`)

**Validation:** If `pattern` is set, the widget compiles it once (on Init or
when the prop changes) and tests the current value on each render. Visual
feedback: border color changes — `success` if valid, `danger` if invalid, theme
default if no pattern.

**Rendering:** Styled box with border. When focused, show cursor character.
When empty and unfocused, show placeholder in muted style. When invalid, border
uses danger color.

**Backed by:** Could wrap `bubbles/textinput` internally, or implement directly
(it's simple enough). Design decision deferred to implementation — the external
contract is the same either way.

---

### textarea (M4-5)

Multi-line text input. Structurally similar to input but with vertical scrolling.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `placeholder` | string | `""` | Shown when value is empty |
| `value` | string | `""` | Current text value |
| `max_lines` | int | 0 | Visible height (0 = grow to content, up to allocated) |
| `style` | token string | `""` | Base style |

**Events emitted:** `change` (on edit), `submit` (configurable — Enter vs Ctrl-Enter)

**Widget state:** cursor row/col, viewport (scroll offset), current value

**Rendering:** Bordered box. Line numbers optional. Scrollbar indicator if
content exceeds visible height. Uses the shared viewport utility.

---

### select (M4-6)

Dropdown/list selection.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `options` | `[{label, value}]` | `[]` | Available choices |
| `selected` | string or `[string]` | `""` | Selected value(s) |
| `multi` | bool | false | Allow multiple selections |
| `filterable` | bool | false | Type to filter options |
| `style` | token string | `""` | Base style |

**Events emitted:**

| Event | When | Data |
|-------|------|------|
| `change` | Selection changes | `{selected}` (string or array) |

**Widget state:** `open` (dropdown visible), `highlightedIndex`, `filterText`,
viewport for long option lists

**Default behaviors:**
- Arrow up/down navigates options
- Enter selects (single) or toggles (multi)
- Type to filter when `filterable: true`
- Escape closes dropdown

**Rendering:**
- Collapsed: shows selected label(s) in a bordered box
- Expanded: vertical list of options, highlighted item has accent bg,
  selected items have checkmark prefix (multi mode)

**Writeback:** On selection change, writes `node.Props["selected"]`.

---

### button (M4-7)

Focusable action trigger. No internal state beyond a transient "pressed"
visual.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `label` | string | `""` | Button text |
| `style` | token string | `""` | Base style |
| `disabled` | bool | false | Grayed out, ignores input |

**Events emitted:**

| Event | When | Data |
|-------|------|------|
| `click` | Enter or Space while focused (and not disabled) | `{}` |

**Visual states:** default, focused (accent border), disabled (dim + muted),
active/pressed (brief inverted colors — 100ms, cosmetic only)

**Rendering:** Centered label in a bordered box. Style varies by state.

---

### table (M4-8)

The most complex leaf widget. Rows + columns with sorting, scrolling, selection,
and optional row expansion.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `columns` | `[{key, label, width?, sortable?}]` | `[]` | Column definitions |
| `rows` | `[{...}]` | `[]` | Row data (objects keyed by column key) |
| `sortable` | bool | false | Enable column sorting globally |
| `expandable` | bool | false | Enable row expansion on Enter |
| `row_style` | string or map | `""` | Per-row style mapping (see below) |
| `style` | token string | `""` | Base style |

**Column width:** Each column can declare a `width` (int = fixed chars,
absent = auto-sized to content, `"fill"` = take remaining). Column width
allocation follows the same algorithm as container child sizing.

**Row styling (`row_style`):**

Two modes:
- **String (script):** A goja expression evaluated per-row. Receives `row` as
  the data object, returns a style token string. Example:
  `"row.status === 'pass' ? 'success' : 'danger'"`. This is the M3 dependency —
  for M4 testing, use the map mode.
- **Map:** A static mapping from a field value to a style token. Example:
  `{"field": "status", "map": {"pass": "success", "fail": "danger"}}`.
  This requires no scripting and works in M4 standalone.

**Events emitted:**

| Event | When | Data |
|-------|------|------|
| `select` | Row highlighted/entered | `{row, index}` |
| `sort` | Column header activated | `{column, direction}` |
| `expand` | Row expanded/collapsed | `{row, index, expanded}` |

**Default behaviors:**
- Arrow up/down scrolls rows
- Enter expands row (if expandable) or emits select
- Column header click/enter sorts by that column (ascending → descending → none)
- Sort is local: the widget reorders its internal view of `rows`, does not
  mutate the DOM prop

**Widget state:**
```
selectedRow  int
scrollOffset int              (viewport)
sortColumn   string
sortAsc      bool
expandedRows map[int]bool
```

**Rendering:**

```
┌──────────────────────────────────────────┐
│ Name          │ Status  │ Duration       │  ← header (bold, header token)
├──────────────────────────────────────────┤
│ test_login    │ ✓ pass  │ 0.3s           │  ← row (success token)
│ test_signup   │ ✗ fail  │ 1.2s           │  ← row (danger token)
│ ▼ test_pay... │ ✗ fail  │ 2.1s           │  ← expanded row
│   Error: ...  │                          │  ← expansion detail
│ test_logout   │ ✓ pass  │ 0.1s           │
│                                          │
│ 4 rows (sorted by Status ▲)             │  ← footer
└──────────────────────────────────────────┘
```

Sort indicator: `▲` or `▼` in the active column header. Expansion detail is
rendered as indented text below the row (content comes from a `detail` field in
the row data, or from a script that populates it — M3 dependency for the script
path, but static `detail` field works without it).

---

### list (M4-9)

Vertical item list with selection, badges, and optional filtering. Similar to
select but meant for display + interaction rather than form input.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `items` | `[{id, label, badge?, style?}]` | `[]` | List items |
| `selected` | string | `""` | Selected item ID |
| `filterable` | bool | false | Type to filter |
| `style` | token string | `""` | Base style |

**Badge:** A short string displayed right-aligned on the item's row, with its
own style. Example: `{"id": "pr-42", "label": "Fix login", "badge": "3 comments", "style": "warning"}`.

**Events emitted:**

| Event | When | Data |
|-------|------|------|
| `select` | Item selected via Enter | `{id, label}` |

**Default behaviors:**
- Arrow up/down navigates
- Type to filter (when `filterable: true`)
- Enter selects

**Widget state:** `selectedIndex`, `filterText`, viewport

**Rendering:**

```
  Fix login bug                   3 comments
▸ Add OAuth support               review       ← selected (accent)
  Update README
```

Selected item has a cursor indicator (`▸`) and accent styling. Badges
right-aligned, styled per-item.

---

### diff (M4-10)

Diff viewer with split and unified modes, hunk navigation, and line numbers.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `hunks` | `[{old_start, new_start, lines}]` | `[]` | Diff hunks |
| `mode` | `"unified"` \| `"split"` | `"unified"` | Display mode |
| `file_name` | string | `""` | Shown in header |
| `style` | token string | `""` | Base style |

**Hunk line format:**
```json
{"type": "add" | "remove" | "context", "content": "...", "old_num": 42, "new_num": 43}
```

**Events emitted:**

| Event | When | Data |
|-------|------|------|
| `select_line` | Enter on a line | `{type, line_num, content}` |
| `hunk_navigate` | n/p pressed | `{hunk_index}` |

**Default behaviors:**
- `d` toggles split ↔ unified mode
- `n` / `p` jump to next / previous hunk
- Arrow keys scroll
- Line numbers always displayed

**Widget state:** `mode`, `currentHunk`, viewport

**Rendering — unified mode:**

```
── handler.go ──────────────────────────────
@@ -40,6 +40,8 @@
  40   40 │ func handleRequest(w http.ResponseWriter, r *http.Request) {
  41   41 │     ctx := r.Context()
       42 │+    if err := validate(r); err != nil {
       43 │+        http.Error(w, err.Error(), 400)
       44 │+        return
       45 │+    }
  42   46 │     data, err := fetchData(ctx)
```

Line numbers: old and new side-by-side in the gutter. Added lines have `add`
token (green), removed have `remove` token (red), context lines are default.

**Rendering — split mode:**

```
── handler.go ──────────────────────────────
 40 │ func handleRequest(...)  │  40 │ func handleRequest(...)
 41 │     ctx := r.Context()   │  41 │     ctx := r.Context()
    │                          │  42 │+    if err := validate(r);
    │                          │  43 │+        http.Error(w, ...)
    │                          │  44 │+        return
    │                          │  45 │+    }
 42 │     data, err := fetch.. │  46 │     data, err := fetch..
```

Split mode divides available width in half (minus gutter). Each side renders
independently with its own line numbers.

---

### log (M4-11)

Append-only scrolling text with ANSI passthrough and severity coloring.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `lines` | `[{text, level?, timestamp?}]` | `[]` | Log entries |
| `auto_scroll` | bool | true | Enable sticky-bottom behavior |
| `max_lines` | int | 1000 | Truncate oldest lines when exceeded |
| `style` | token string | `""` | Base style |

**Severity levels and colors:**

| Level | Color | Token |
|-------|-------|-------|
| `error` | red | `danger` |
| `warn` | yellow | `warning` |
| `info` | default | (none) |
| `debug` | gray | `muted` |

**Special patch semantics:** When a `patch` operation updates a log node with an
`append_lines` prop, the lines are appended to the existing `lines` array rather
than replacing it. This is the only widget with non-standard patch merge behavior.
Implementation: the patch engine doesn't know about this — the log widget handles
it in its prop reading logic. On `View()`, it reads both `lines` and
`append_lines` from props, appends, and clears `append_lines`.

Actually, better approach: this append semantic should be handled at the DOM/patch
layer via a convention. When `update` encounters an `append_lines` key on a log
node, it appends to `lines` and removes `append_lines`. This keeps widgets
stateless w.r.t. prop reading. **Decision: handle in patch layer as a
widget-type-aware merge rule.** Document this as a patch engine extension point.

**ANSI passthrough:** Log lines may contain raw ANSI escape codes (from test
runners, build tools). The log widget must NOT strip or re-process these. Lip
Gloss styling is applied around the line (e.g., timestamp prefix), but the line
content itself is rendered verbatim.

**Sticky-bottom:** Uses the viewport utility with `sticky: true`. Auto-scrolls
to bottom on new lines. If the user scrolls up, sticky disengages. Scrolling
back to the bottom re-engages.

**Widget state:** viewport (with sticky-bottom)

**Rendering:**

```
10:32:01 [INFO]  Starting test suite...
10:32:01 [INFO]  Running test_login...
10:32:02 [PASS]  test_login (0.3s)
10:32:02 [INFO]  Running test_signup...
10:32:03 [FAIL]  test_signup (1.2s)          ← danger token
10:32:03 [ERROR] expected 200, got 500       ← danger token, bold
```

Timestamp is optional (only shown if present in line data). Level badge is
colored by severity.

---

### code (M4-12)

Syntax-highlighted code block with line numbers and line-range highlighting.

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `content` | string | `""` | Source code |
| `language` | string | `""` | Language for syntax highlighting |
| `line_numbers` | bool | true | Show line number gutter |
| `highlight_lines` | `[int]` or `["3-7"]` | `[]` | Lines to highlight |
| `start_line` | int | 1 | Line number offset (for showing a snippet from line 42) |
| `style` | token string | `""` | Base style |

**Syntax highlighting:** Uses [Chroma](https://github.com/alecthomas/chroma)
for tokenization, then maps Chroma token types to Lip Gloss styles. The theme
defines a Chroma-to-Lip Gloss mapping. If the language is unknown or empty,
content is rendered without highlighting.

**Line highlighting:** `highlight_lines` accepts individual line numbers and
ranges (`"3-7"`). Highlighted lines get a subtle background accent (e.g.,
slightly brighter background) to draw attention without obscuring syntax colors.

**Default behaviors:**
- Arrow keys scroll
- `y` yanks the highlighted line range to clipboard via OSC 52 escape sequence

**Widget state:** viewport

**Rendering:**

```
  42 │ func handleRequest(w http.ResponseWriter, r *http.Request) {
  43 │     ctx := r.Context()
  44 │     if err := validate(r); err != nil {          ← highlighted bg
  45 │         http.Error(w, err.Error(), 400)          ← highlighted bg
  46 │         return                                   ← highlighted bg
  47 │     }                                            ← highlighted bg
  48 │     data, err := fetchData(ctx)
```

Line numbers right-aligned in a gutter, separated by `│`. Gutter uses
`line-number` style token (muted).

---

## Cross-cutting concerns

### Prop validation

Each widget validates its props on Init and on each View call (props may change
between renders). Validation is lenient:

- Missing optional props → use default
- Wrong type for a prop → use default, log a warning (do not crash)
- Unknown props → ignore (forward-compatible)
- Missing required prop → render an error placeholder in the widget's output
  (e.g., `[table: missing "columns" prop]`)

No shared `PropSchema` registry for v1. Each widget handles its own validation
internally. If we find ourselves duplicating logic, we can extract a shared
validator later.

### Empty / error states

Every widget must handle:
- **Empty data:** table with no rows, list with no items, log with no lines →
  render a centered, muted "(empty)" placeholder
- **Prop errors:** as above, render an inline error message identifying the
  widget and what's wrong. Example: `[input "search_box": pattern "([" is not valid regex]`
- **Zero dimensions:** width=0 or height=0 → return empty string (don't panic)

### Focus model

Focus is not per-widget state — it's a single `focusedID string` managed by the
M5 runtime and passed down via `ViewContext.Focused`. Widgets use it to adjust
their visual rendering (border highlight, cursor visibility). Widgets don't
decide if they're focused; they're told.

Container widgets participate in focus management by maintaining a focus ring of
descendant focusable nodes. Which nodes are focusable:
- input, textarea, select, button: always focusable
- table, list: focusable (for row navigation)
- diff, code, log: focusable (for scroll navigation)
- text: not focusable
- container: not directly focusable (focus goes to children)

### Event routing and default behaviors

When a widget's `Update()` produces an `Event`, the M5 runtime routes it:

1. Check if the source node has a script hook for this event type
   (`Scripts["on_change"]`, etc.)
2. If script exists → run it (M3 responsibility). The widget's default behavior
   for this event is **skipped**.
3. If no script → the widget's default behavior was already applied during
   `Update()`. No further action unless the event is `"claude"` routed.
4. If the script body is literally `"claude"` → enqueue in EventQueue.

This means widgets implement their default behaviors directly in `Update()`.
Scripts override by replacing the hook. The runtime layer (M5) checks for
scripts after `Update()` returns.

For M4 testing, there are no scripts — all behaviors are defaults, and events
can be asserted directly from `UpdateResult`.

### The append_lines patch extension

The log widget needs append semantics. This requires the patch engine to know
about it. Proposed extension:

```go
// In the patch engine's update operation:
// If the node type is "log" and the update props contain "append_lines",
// append those lines to the existing "lines" prop instead of replacing.
```

This is a small, targeted change to the existing `Tree.Patch()` method. It can
be implemented as a widget-type-aware merge hook:

```go
// MergeHook is called during patch update to customize prop merging.
// If it returns true, the hook handled the prop; the patch engine skips
// its default merge for that key.
type MergeHook func(node *dom.Node, key string, value any) bool
```

The log widget registers a hook that handles `append_lines`. For M4, this hook
lives in the widget package but is injected into the patch engine. This keeps
the DOM layer clean of widget-specific knowledge while enabling the behavior.

---

## Testing strategy

### Golden file tests (every widget)

Each widget gets golden file tests in `testdata/golden/widget_<type>/`. The test
pattern:

```go
func TestTextWidget_GoldenFiles(t *testing.T) {
    tests := []struct {
        name  string
        node  *dom.Node   // node with specific props
        width int
        focus bool
    }{
        {"plain", textNode("hello"), 40, false},
        {"styled", styledTextNode("bold info"), 40, false},
        {"wrapped", longTextNode(), 20, false},
        {"segments", segmentedTextNode(), 40, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            w := NewTextWidget()
            w.Init(tt.node)
            got := w.View(tt.node, nil, ViewContext{Width: tt.width, Focused: tt.focus, Theme: DefaultTheme()})
            testutil.GoldenFile(t, tt.name, []byte(got))
        })
    }
}
```

Width is always fixed in golden tests (typically 40 or 80 columns) to ensure
deterministic output. The `GOLDEN_UPDATE=1` mechanism from testutil is used.

### Behavior tests (interactive widgets)

For widgets with Update logic (input, table, select, etc.), table-driven tests
assert:

- Given this widget state + this tea.Msg → assert new state + events produced
- No terminal, no BubbleTea program — just direct widget method calls

### Registry and pipeline tests

- Unknown node type → error
- Two-pass render produces correct composed output for nested containers
- Tree sync creates/removes instances correctly

---

## Ticket breakdown and ordering

```
M4-1: Registry, pipeline, theme, utilities
  ↓
M4-2: container  ← needed by all integration/composition tests
  ↓
M4-3: text       ← simplest leaf, validates the pipeline
  ↓  (M4-4 through M4-12 can proceed in parallel after M4-3 proves the pattern)
  ├─ M4-4: input
  ├─ M4-5: textarea
  ├─ M4-6: select
  ├─ M4-7: button
  ├─ M4-8: table
  ├─ M4-9: list
  ├─ M4-10: diff
  ├─ M4-11: log (+ append_lines patch hook)
  └─ M4-12: code (+ chroma dependency)
```

M4-1 must come first (it defines the interfaces everyone implements). M4-2
(container) comes next because composition testing requires it. M4-3 (text) is
the simplest leaf widget and proves the render pipeline end-to-end. After that,
the remaining widgets are independent of each other and can be built in any
order or in parallel.

### Dependencies on other milestones

| Dependency | Where it surfaces | Workaround for M4 |
|------------|-------------------|-------------------|
| M3 (scripts) | `row_style` script mode in table, computed props, event hooks | Use static map mode for row_style; test events via UpdateResult without script routing |
| M2 (MCP) | `append_lines` patch semantics for log | Implement merge hook in widget package, inject into patch engine |
| M1 (DOM) | `dom.Node`, `dom.Tree`, `dom.NodeType` | Already complete ✓ |

---

## Open design questions

1. **Bubbles wrapping vs custom implementation:** For input, textarea, and
   select, do we wrap the corresponding Bubbles components or implement from
   scratch? Wrapping is faster but means adapting to Bubbles' API and managing
   two layers of state. Custom is more work but gives full control over rendering
   and state. **Recommendation:** Start by wrapping Bubbles for input/textarea
   (well-tested, hard to get cursor/unicode right), implement select and table
   custom (Bubbles' list and table have opinions that may conflict with our prop
   model).

2. **Chroma theme mapping:** How detailed should the Chroma→Lip Gloss mapping
   be? Chroma has ~80 token types. We could map all of them or collapse to ~10
   categories (keyword, string, comment, number, operator, etc.).
   **Recommendation:** Start with ~10 categories. Expand if users notice missing
   distinctions.

3. **Terminal width unit:** Should percentage-width children use the container's
   allocated width or the terminal width as the 100% base?
   **Recommendation:** Container's allocated width. This makes nesting
   predictable (50% of a 50% container = 25% of terminal).

4. **Diff rendering complexity:** Split mode is substantially harder than unified
   (synchronized scrolling, line alignment for changed blocks). Should split mode
   be deferred to a fast-follow?
   **Recommendation:** Implement unified first, split as a follow-up within M4.
   The `mode` prop and `d` toggle can exist from the start; split rendering lands
   when it's ready.

5. **OSC 52 clipboard:** Not all terminals support it. Should the code widget's
   `y` to yank silently fail if unsupported?
   **Recommendation:** Yes, silently emit the OSC 52 sequence. Terminals that
   support it will copy; others will ignore it. No error handling needed.

---

## Decisions (noted for review)

1. **`append_lines` for log** — Needs append-not-replace semantics on patch.
   Proposed: a `MergeHook` injected from widget into the patch engine. Keeps DOM
   layer free of widget knowledge but does touch M1/M2 code.

2. **Bubbles wrapping vs custom** — Wrap Bubbles textinput/textarea (cursor and
   unicode are hard to get right). Build table, select, and list custom (Bubbles'
   opinions conflict with our prop model).

3. **Percentage width base** — Percentages are relative to the parent container's
   allocated width, not terminal width. Nesting is predictable: 50% inside 50%
   = 25% of terminal. May surprise Claude; document in tool schema.

4. **Split diff** — Build unified first, split as a follow-up within M4. The
   `mode` prop and `d` toggle exist from the start; split rendering lands later.

5. **Focus is not widget state** — A single `focusedID` string is owned by the
   runtime and passed down via ViewContext. Widgets are told; they don't decide.
