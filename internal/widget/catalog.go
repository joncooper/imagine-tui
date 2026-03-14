package widget

// Info describes a widget type for LLM discovery.
type Info struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Props       []PropInfo  `json:"props"`
	Events      []EventInfo `json:"events,omitempty"`
	Focusable   bool        `json:"focusable"`
	Scrollable  bool        `json:"scrollable"`
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
				{Name: "item_template", Type: "map", Description: "Template for set_items: a node spec with {{key}} placeholders in string props"},
			},
			Focusable:  false,
			Scrollable: true,
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
			Focusable:  true,
			Scrollable: true,
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
			Focusable:  true,
			Scrollable: true,
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
			Focusable:  true,
			Scrollable: false,
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
