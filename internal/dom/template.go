package dom

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExpandTemplate expands a template NodeSpec for each item in the data slice.
// parentID is used as a prefix for generated IDs.
// Each item is a map[string]any. If an item has a "key" field (string), it is
// used in the generated ID: "{parentID}-{key}". Otherwise, the 0-based index
// is used: "{parentID}-{index}".
func ExpandTemplate(parentID string, tmpl *NodeSpec, items []map[string]any) ([]*NodeSpec, error) {
	return ExpandTemplateAt(parentID, tmpl, items, 0)
}

// ExpandTemplateAt is like ExpandTemplate but starts index-based IDs at offset.
// This is used by AppendItems to avoid ID collisions with existing children.
func ExpandTemplateAt(parentID string, tmpl *NodeSpec, items []map[string]any, offset int) ([]*NodeSpec, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("expand template: template is nil")
	}
	if len(items) == 0 {
		return nil, nil
	}

	specs := make([]*NodeSpec, 0, len(items))
	for i, item := range items {
		suffix := itemSuffix(item, i+offset)
		idPrefix := parentID + "-" + suffix
		spec := expandSpec(idPrefix, tmpl, item, 0)
		specs = append(specs, spec)
	}
	return specs, nil
}

// ParseItemTemplate extracts and validates the item_template prop from a Node.
// Returns the template as a *NodeSpec, or an error if the prop is missing or malformed.
func ParseItemTemplate(node *Node) (*NodeSpec, error) {
	raw, ok := node.Props["item_template"]
	if !ok {
		return nil, fmt.Errorf("node %q: missing item_template prop", node.ID)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("node %q: item_template must be an object, got %T", node.ID, raw)
	}

	// Marshal to JSON and back to get a proper NodeSpec.
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("node %q: marshal item_template: %w", node.ID, err)
	}
	var spec NodeSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("node %q: parse item_template: %w", node.ID, err)
	}
	return &spec, nil
}

// expandSpec clones a template spec, assigns IDs, and replaces placeholders.
func expandSpec(idPrefix string, tmpl *NodeSpec, item map[string]any, childIndex int) *NodeSpec {
	spec := &NodeSpec{
		ID:       idPrefix,
		Type:     tmpl.Type,
		Props:    replacePlaceholders(deepCopyProps(tmpl.Props), item),
		Scripts:  copyStrMap(tmpl.Scripts),
		Computed: copyStrMap(tmpl.Computed),
	}
	if len(tmpl.Children) > 0 {
		spec.Children = make([]*NodeSpec, len(tmpl.Children))
		for i, child := range tmpl.Children {
			childID := idPrefix + "-" + strconv.Itoa(i)
			spec.Children[i] = expandSpec(childID, child, item, i)
		}
	}
	return spec
}

// itemSuffix returns the ID suffix for an item: the "key" field if present, else the index.
func itemSuffix(item map[string]any, index int) string {
	if key, ok := item["key"]; ok {
		if s, ok := key.(string); ok && s != "" {
			return s
		}
	}
	return strconv.Itoa(index)
}

// replaceInString performs {{key}} replacement on a single string.
func replaceInString(s string, item map[string]any) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	var b strings.Builder
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			b.WriteString(s)
			break
		}
		end := strings.Index(s[start:], "}}")
		if end < 0 {
			b.WriteString(s)
			break
		}
		end += start

		b.WriteString(s[:start])
		key := s[start+2 : end]
		if item != nil {
			if val, ok := item[key]; ok {
				fmt.Fprintf(&b, "%v", val)
			}
		}
		s = s[end+2:]
	}
	return b.String()
}

// replacePlaceholders walks all string values in a map and replaces {{key}} patterns.
func replacePlaceholders(props, item map[string]any) map[string]any {
	if props == nil {
		return nil
	}
	for k, v := range props {
		if s, ok := v.(string); ok {
			props[k] = replaceInString(s, item)
		}
	}
	return props
}

// deepCopyProps makes a deep copy of a props map.
func deepCopyProps(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	return copyMap(m)
}

// copyStrMap copies a string→string map.
func copyStrMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	return copyStringMap(m)
}
