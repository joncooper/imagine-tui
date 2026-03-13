package widget

import (
	"github.com/joncooper/imagine-tui/internal/dom"
)

// PropString returns the string prop at key, or fallback if missing or wrong type.
func PropString(node *dom.Node, key, fallback string) string {
	v, ok := node.GetProp(key)
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}

// PropInt returns the int prop at key, or fallback if missing or wrong type.
// Handles both int and float64 (JSON numbers unmarshal as float64).
func PropInt(node *dom.Node, key string, fallback int) int {
	v, ok := node.GetProp(key)
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return fallback
	}
}

// PropBool returns the bool prop at key, or fallback if missing or wrong type.
func PropBool(node *dom.Node, key string, fallback bool) bool {
	v, ok := node.GetProp(key)
	if !ok {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

// PropStringSlice returns the []string prop at key. Non-string elements are skipped.
// Returns nil if missing or wrong type.
func PropStringSlice(node *dom.Node, key string) []string {
	v, ok := node.GetProp(key)
	if !ok {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// PropMapSlice returns the []map[string]any prop at key.
// Returns nil if missing or wrong type.
func PropMapSlice(node *dom.Node, key string) []map[string]any {
	v, ok := node.GetProp(key)
	if !ok {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}
