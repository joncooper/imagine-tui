package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func loadSetItemsItems(input setItemsInput) ([]map[string]any, error) {
	if strings.TrimSpace(input.File) != "" {
		return nil, fmt.Errorf("set_items: file-based loading is not supported; use imagine-tui push-items --socket <socket-path> --target %q --file %q from the caller side or pass items inline", input.Target, input.File)
	}
	if len(input.Items) == 0 {
		return nil, fmt.Errorf("set_items: missing required parameter: items")
	}

	var items []map[string]any
	if err := json.Unmarshal(input.Items, &items); err != nil {
		return nil, fmt.Errorf("invalid items: %v", err)
	}
	return items, nil
}
