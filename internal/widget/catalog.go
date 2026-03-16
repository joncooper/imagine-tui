package widget

// Info describes a widget type for LLM discovery.
type Info struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Props       []PropInfo  `json:"props"`
	Events      []EventInfo `json:"events,omitempty"`
	Focusable   bool        `json:"focusable"`
	Scrollable  bool        `json:"scrollable"`
	KeyBindings []string    `json:"key_bindings,omitempty"`
	FocusModel  string      `json:"focus_model,omitempty"`
	UsageNotes  []string    `json:"usage_notes,omitempty"`
	Example     string      `json:"example,omitempty"`
}

// PropInfo describes a widget prop.
type PropInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "int", "bool", "array", "map"
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
}

// EventInfo describes an event a widget can emit.
type EventInfo struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	DataFields  map[string]string `json:"data_fields,omitempty"`
}

// Catalog returns metadata for all available widget types.
func Catalog() []Info {
	return []Info{
		{
			Type:        "container",
			Description: "Layout container that arranges children horizontally or vertically. Use as the structural backbone of any UI. When overflow is set to \"scroll\" and height is constrained, the container becomes keyboard-scrollable.",
			Props: []PropInfo{
				{Name: "direction", Type: "string", Description: "Layout direction: \"horizontal\" or \"vertical\"", Default: "vertical"},
				{Name: "gap", Type: "int", Description: "Spacing between children in cells"},
				{Name: "padding", Type: "int", Description: "Inner padding in cells"},
				{Name: "border", Type: "string", Description: "Border style: \"none\", \"rounded\", \"thick\", \"double\", \"hidden\"", Default: "none"},
				{Name: "width", Type: "string", Description: "Width: integer (fixed), \"50%\" (percent), or \"fill\" (remaining space)"},
				{Name: "height", Type: "string", Description: "Height: integer (fixed) or \"50%\" of the available parent height"},
				{Name: "max_height", Type: "string", Description: "Maximum height: integer (fixed) or \"50%\" of the available parent height"},
				{Name: "overflow", Type: "string", Description: "Overflow behavior. Set to \"scroll\" to enable a viewport when height is constrained"},
				{Name: "style", Type: "string", Description: "Theme token for styling"},
				{Name: "focus_trap", Type: "bool", Description: "If true, Tab/Shift-Tab cycles only among focusable descendants"},
				{Name: "initial_focus", Type: "string", Description: "Root-only hint: focus this node ID after layout/replace before falling back to DOM order"},
				{Name: "item_template", Type: "map", Description: "Template for set_items: a node spec with {{key}} placeholders in string props"},
			},
			Focusable:   false,
			Scrollable:  true,
			KeyBindings: []string{"Tab/Shift-Tab: cycle focus among focusable descendants", "Up/Down: scroll only when overflow is \"scroll\" and the container is focused"},
			FocusModel:  "Containers are not in the Tab order by default. A scroll container becomes focusable when overflow is \"scroll\" and height/max_height is constrained. On the root container, set initial_focus to a focusable node ID to override DOM-order focus after layout.",
			UsageNotes: []string{
				"layout replaces the whole tree. Run layout before set_items when possible.",
				"If you need to experiment with structure and come back, take a snapshot first and restore it later.",
				"Without a height or max_height constraint, tall content can push siblings off-screen. Use height + overflow: \"scroll\" for local scrolling panes.",
			},
			Example: "Five-widget example:\n{\n  \"id\": \"root\",\n  \"type\": \"container\",\n  \"props\": {\"direction\": \"vertical\", \"gap\": 1, \"initial_focus\": \"results_table\"},\n  \"children\": [\n    {\"id\": \"header\", \"type\": \"text\", \"props\": {\"content\": \"Log Viewer\"}},\n    {\"id\": \"search\", \"type\": \"input\", \"props\": {\"placeholder\": \"Search logs\"}},\n    {\"id\": \"toolbar\", \"type\": \"container\", \"props\": {\"direction\": \"horizontal\", \"gap\": 1}, \"children\": [\n      {\"id\": \"refresh\", \"type\": \"button\", \"props\": {\"label\": \"Refresh\"}},\n      {\"id\": \"status\", \"type\": \"text\", \"props\": {\"content\": \"Ready\"}}\n    ]},\n    {\"id\": \"results_table\", \"type\": \"table\", \"props\": {\"columns\": [{\"key\": \"level\", \"label\": \"Level\"}, {\"key\": \"msg\", \"label\": \"Message\"}], \"rows\": []}},\n    {\"id\": \"footer\", \"type\": \"text\", \"props\": {\"content\": \"Tab cycles focus\"}}\n  ]\n}",
		},
		{
			Type:        "text",
			Description: "Styled text display. Supports plain text, styled segments, and word wrapping.",
			Props: []PropInfo{
				{Name: "content", Type: "string", Description: "Text to display (alias: \"text\")"},
				{Name: "segments", Type: "array", Description: "Styled segments: [{\"text\": \"...\", \"style\": \"bold\"}]"},
				{Name: "style", Type: "string", Description: "Theme token: \"bold\", \"dim\", \"danger\", \"warning\", \"success\", \"muted\""},
				{Name: "wrap", Type: "bool", Description: "Word wrap long lines", Default: true},
				{Name: "max_lines", Type: "int", Description: "Truncate to N lines"},
			},
			Focusable:  false,
			Scrollable: false,
		},
		{
			Type:        "list",
			Description: "Navigable item list with selection, badges, and optional filtering. User navigates with up/down arrows and selects with Enter.",
			Props: []PropInfo{
				{Name: "items", Type: "array", Description: "Array of {id, label, badge, style} objects. Use set_items tool to populate efficiently."},
				{Name: "selected", Type: "string", Description: "ID of initially selected item"},
				{Name: "filterable", Type: "bool", Description: "Enable type-to-filter", Default: false},
			},
			Events: []EventInfo{
				{Type: "select", Description: "User pressed Enter on an item", DataFields: map[string]string{"id": "Item ID", "label": "Item label"}},
			},
			Focusable:   true,
			Scrollable:  true,
			KeyBindings: []string{"Up/Down: move selection", "Enter: emit select event"},
			FocusModel:  "Arrow keys work only when the list has focus. Focus starts on the first focusable node in DOM order unless the root container sets initial_focus. Tab/Shift-Tab cycle focus.",
		},
		{
			Type:        "table",
			Description: "Sortable data table with row selection and expandable rows. User navigates rows with up/down, selects with Enter.",
			Props: []PropInfo{
				{Name: "columns", Type: "array", Description: "Column definitions: [{key, label, width (int), sortable (bool)}]"},
				{Name: "rows", Type: "array", Description: "Row data: [{col_key: value, ...}]. Use set_items tool to populate efficiently."},
				{Name: "expandable", Type: "bool", Description: "Enable expand/collapse on Enter. Rows need a \"detail\" field.", Default: false},
				{Name: "row_style", Type: "map", Description: "Conditional row styling: {field: \"status\", map: {\"error\": \"danger\"}}"},
			},
			Events: []EventInfo{
				{Type: "select", Description: "User pressed Enter on a row (when not expandable)", DataFields: map[string]string{"index": "Row index", "row": "Full row data"}},
				{Type: "expand", Description: "User toggled row expansion", DataFields: map[string]string{"index": "Row index", "expanded": "New expanded state"}},
			},
			Focusable:   true,
			Scrollable:  true,
			KeyBindings: []string{"Up/Down: move selected row", "Enter: select row or toggle expansion"},
			FocusModel:  "Arrow keys work only when the table has focus. Focus starts on the first focusable node in DOM order unless the root container sets initial_focus. Tab/Shift-Tab cycle focus.",
			UsageNotes:  []string{"If the table should own arrow keys immediately after layout, set root props.initial_focus to the table ID.", "A tall table without a constrained parent can push siblings off-screen; wrap it in a container with height + overflow: \"scroll\" when needed."},
		},
		{
			Type:        "input",
			Description: "Single-line text input with cursor, validation, and placeholder.",
			Props: []PropInfo{
				{Name: "value", Type: "string", Description: "Current input value"},
				{Name: "placeholder", Type: "string", Description: "Placeholder text shown when empty"},
				{Name: "pattern", Type: "string", Description: "Regex for validation. Border turns green/red."},
				{Name: "style", Type: "string", Description: "Theme token"},
			},
			Events: []EventInfo{
				{Type: "submit", Description: "User pressed Enter", DataFields: map[string]string{"value": "Current input value"}},
				{Type: "change", Description: "Input value changed", DataFields: map[string]string{"value": "New value"}},
			},
			Focusable:   true,
			Scrollable:  false,
			KeyBindings: []string{"Text keys: edit value", "Enter: emit submit event", "Tab/Shift-Tab: leave the input and move focus"},
			FocusModel:  "Inputs are focusable by default and consume text-entry keys while focused.",
		},
		{
			Type:        "textarea",
			Description: "Multi-line text editor with cursor movement.",
			Props: []PropInfo{
				{Name: "value", Type: "string", Description: "Current text content"},
				{Name: "placeholder", Type: "string", Description: "Placeholder text"},
				{Name: "max_lines", Type: "int", Description: "Height constraint in lines"},
			},
			Events: []EventInfo{
				{Type: "change", Description: "Text content changed", DataFields: map[string]string{"value": "New value"}},
			},
			Focusable:  true,
			Scrollable: false,
		},
		{
			Type:        "button",
			Description: "Focusable action trigger. Activated with Enter or Space.",
			Props: []PropInfo{
				{Name: "label", Type: "string", Description: "Button text"},
				{Name: "disabled", Type: "bool", Description: "Disable the button", Default: false},
				{Name: "style", Type: "string", Description: "Theme token"},
			},
			Events: []EventInfo{
				{Type: "click", Description: "User pressed Enter or Space on the button"},
			},
			Focusable:  true,
			Scrollable: false,
		},
		{
			Type:        "select",
			Description: "Dropdown select with single or multi-select, optional filtering.",
			Props: []PropInfo{
				{Name: "options", Type: "array", Description: "Options: [{label, value}]"},
				{Name: "selected", Type: "string", Description: "Selected value (string for single, array for multi)"},
				{Name: "multi", Type: "bool", Description: "Enable multi-select", Default: false},
				{Name: "filterable", Type: "bool", Description: "Enable type-to-filter options", Default: false},
				{Name: "style", Type: "string", Description: "Theme token"},
			},
			Events: []EventInfo{
				{Type: "change", Description: "Selection changed", DataFields: map[string]string{"selected": "Selected value(s)"}},
			},
			Focusable:  true,
			Scrollable: false,
		},
		{
			Type:        "log",
			Description: "Append-only scrolling log with severity coloring and auto-scroll.",
			Props: []PropInfo{
				{Name: "lines", Type: "array", Description: "Log lines: [{text, level (\"error\"/\"warn\"/\"debug\"), timestamp}]"},
				{Name: "auto_scroll", Type: "bool", Description: "Stick to bottom on new lines", Default: true},
				{Name: "max_lines", Type: "int", Description: "Maximum retained lines", Default: 1000},
			},
			Focusable:  true,
			Scrollable: true,
		},
		{
			Type:        "code",
			Description: "Code block with line numbers and line highlighting. Scrollable.",
			Props: []PropInfo{
				{Name: "content", Type: "string", Description: "Code text"},
				{Name: "line_numbers", Type: "bool", Description: "Show line numbers", Default: true},
				{Name: "start_line", Type: "int", Description: "Starting line number", Default: 1},
				{Name: "highlight_lines", Type: "array", Description: "Lines to highlight: [3, \"5-8\", 12]"},
			},
			Focusable:  true,
			Scrollable: true,
		},
		{
			Type:        "progress",
			Description: "Progress bar with an optional label and percentage display.",
			Props: []PropInfo{
				{Name: "value", Type: "int", Description: "Progress percentage from 0 to 100"},
				{Name: "label", Type: "string", Description: "Optional label rendered before the bar"},
				{Name: "show_percent", Type: "bool", Description: "Show the numeric percentage", Default: true},
				{Name: "style", Type: "string", Description: "Theme token for label styling and bar color selection"},
			},
			Focusable:  false,
			Scrollable: false,
		},
		{
			Type:        "spinner",
			Description: "Animated spinner using Bubble Tea's spinner component.",
			Props: []PropInfo{
				{Name: "label", Type: "string", Description: "Optional label rendered after the spinner"},
				{Name: "active", Type: "bool", Description: "Whether the spinner is animating", Default: true},
				{Name: "spinner", Type: "string", Description: "Preset spinner style: \"line\", \"dot\", \"mini_dot\", \"jump\", \"pulse\", \"points\", \"globe\", \"moon\", \"monkey\", \"meter\", \"hamburger\", or \"ellipsis\"", Default: "line"},
				{Name: "style", Type: "string", Description: "Theme token applied to the spinner output"},
			},
			Focusable:  false,
			Scrollable: false,
		},
		{
			Type:        "markdown",
			Description: "Rendered markdown content using glamour styling.",
			Props: []PropInfo{
				{Name: "content", Type: "string", Description: "Markdown source text"},
				{Name: "theme", Type: "string", Description: "Glamour theme: \"ascii\", \"dark\", \"light\", or \"notty\"", Default: "ascii"},
			},
			Focusable:  false,
			Scrollable: false,
		},
		{
			Type:        "sparkline",
			Description: "Compact inline trend line rendered from numeric values.",
			Props: []PropInfo{
				{Name: "values", Type: "array", Description: "Numeric series to render"},
				{Name: "label", Type: "string", Description: "Optional label rendered before the sparkline"},
				{Name: "min", Type: "int", Description: "Optional lower bound for scaling"},
				{Name: "max", Type: "int", Description: "Optional upper bound for scaling"},
				{Name: "style", Type: "string", Description: "Theme token applied to the rendered output"},
			},
			Focusable:  false,
			Scrollable: false,
		},
		{
			Type:        "diff",
			Description: "Unified or split diff viewer with hunk navigation.",
			Props: []PropInfo{
				{Name: "hunks", Type: "array", Description: "Diff hunks: [{old_start, new_start, lines: [{type (\"add\"/\"remove\"/\"context\"), content, old_num, new_num}]}]"},
				{Name: "file_name", Type: "string", Description: "File name to display"},
				{Name: "mode", Type: "string", Description: "Display mode: \"unified\" or \"split\"", Default: "unified"},
			},
			Events: []EventInfo{
				{Type: "select_line", Description: "User pressed Enter on a line", DataFields: map[string]string{"line": "Line content", "type": "Line type", "line_number": "Line number"}},
				{Type: "hunk_navigate", Description: "User navigated to a hunk", DataFields: map[string]string{"hunk_index": "Hunk index", "direction": "next or prev"}},
			},
			Focusable:  true,
			Scrollable: true,
		},
	}
}

// CatalogMap returns the catalog indexed by widget type for O(1) lookup.
func CatalogMap() map[string]Info {
	m := make(map[string]Info)
	for _, w := range Catalog() {
		m[w.Type] = w
	}
	return m
}
