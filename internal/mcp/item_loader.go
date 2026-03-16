package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadItems(input setItemsInput) ([]map[string]any, error) {
	hasItems := len(input.Items) > 0
	hasFile := strings.TrimSpace(input.File) != ""

	switch {
	case hasItems && hasFile:
		return nil, fmt.Errorf("set_items: provide either items or file, not both")
	case !hasItems && !hasFile:
		return nil, fmt.Errorf("set_items: missing required parameter: items or file")
	case hasItems:
		var items []map[string]any
		if err := json.Unmarshal(input.Items, &items); err != nil {
			return nil, fmt.Errorf("invalid items: %v", err)
		}
		return items, nil
	default:
		return loadItemsFromFile(input.File, input.Format)
	}
}

func loadItemsFromFile(path, format string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("set_items: read file %q: %w", path, err)
	}

	format = normalizeItemFormat(format)
	switch format {
	case "", "auto":
		return loadItemsAuto(path, data)
	case "json":
		return loadItemsJSON(path, data)
	case "jsonl", "ndjson":
		return loadItemsJSONL(path, data)
	default:
		return nil, fmt.Errorf("set_items: unsupported file format %q for %q", format, path)
	}
}

func normalizeItemFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

func loadItemsAuto(path string, data []byte) ([]map[string]any, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return loadItemsJSON(path, data)
	case ".jsonl", ".ndjson":
		return loadItemsJSONL(path, data)
	}

	if items, err := loadItemsJSON(path, data); err == nil {
		return items, nil
	}
	if items, err := loadItemsJSONL(path, data); err == nil {
		return items, nil
	}

	if looksLikeRawLog(data) {
		return nil, fmt.Errorf("set_items: raw log parsing is not supported yet for %q; convert the file to JSON or JSONL first", path)
	}
	return nil, fmt.Errorf("set_items: could not detect a supported format for %q; supported formats are json, jsonl, and ndjson", path)
}

func loadItemsJSON(path string, data []byte) ([]map[string]any, error) {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("set_items: parse json file %q: %w", path, err)
	}
	if items == nil {
		return []map[string]any{}, nil
	}
	return items, nil
}

func loadItemsJSONL(path string, data []byte) ([]map[string]any, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	items := make([]map[string]any, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("set_items: parse jsonl file %q line %d: %w", path, lineNo, err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("set_items: scan jsonl file %q: %w", path, err)
	}
	return items, nil
}

func looksLikeRawLog(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return false
	}
	return !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{")
}
