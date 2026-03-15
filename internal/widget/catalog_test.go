package widget

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestCatalogCoversRegisteredTypes(t *testing.T) {
	registry := DefaultRegistry()
	catalog := CatalogMap()

	// Every type in the registry should have a catalog entry.
	coreTypes := []dom.NodeType{
		dom.TypeContainer, dom.TypeText, dom.TypeInput, dom.TypeTextarea,
		dom.TypeSelect, dom.TypeButton, dom.TypeTable, dom.TypeList,
		dom.TypeDiff, dom.TypeLog, dom.TypeCode, dom.TypeProgress, dom.TypeSpinner,
		dom.TypeMarkdown, dom.TypeSparkline,
	}

	for _, nt := range coreTypes {
		// Verify it's in the registry.
		if _, err := registry.Create(nt); err != nil {
			t.Errorf("type %q not in registry: %v", nt, err)
			continue
		}
		// Verify it's in the catalog.
		info, ok := catalog[string(nt)]
		if !ok {
			t.Errorf("type %q registered but missing from catalog", nt)
			continue
		}
		if info.Description == "" {
			t.Errorf("type %q has empty description", nt)
		}
		if len(info.Props) == 0 {
			t.Errorf("type %q has no props documented", nt)
		}
	}
}

func TestCatalogNoDuplicateTypes(t *testing.T) {
	seen := make(map[string]bool)
	for _, info := range Catalog() {
		if seen[info.Type] {
			t.Errorf("duplicate catalog entry for type %q", info.Type)
		}
		seen[info.Type] = true
	}
}

func TestCatalogPropsHaveRequiredFields(t *testing.T) {
	for _, info := range Catalog() {
		for _, prop := range info.Props {
			if prop.Name == "" {
				t.Errorf("widget %q has prop with empty name", info.Type)
			}
			if prop.Type == "" {
				t.Errorf("widget %q prop %q has empty type", info.Type, prop.Name)
			}
			if prop.Description == "" {
				t.Errorf("widget %q prop %q has empty description", info.Type, prop.Name)
			}
		}
	}
}

func TestCatalogEventsHaveRequiredFields(t *testing.T) {
	for _, info := range Catalog() {
		for _, evt := range info.Events {
			if evt.Type == "" {
				t.Errorf("widget %q has event with empty type", info.Type)
			}
			if evt.Description == "" {
				t.Errorf("widget %q event %q has empty description", info.Type, evt.Type)
			}
		}
	}
}

func TestCatalogMap(t *testing.T) {
	m := CatalogMap()
	if _, ok := m["list"]; !ok {
		t.Error("CatalogMap missing 'list'")
	}
	if _, ok := m["table"]; !ok {
		t.Error("CatalogMap missing 'table'")
	}
}
