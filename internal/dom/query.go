package dom

import "fmt"

// QueryResult holds the state of a single node as returned by Query.
type QueryResult struct {
	ID       string            `json:"id"`
	Type     NodeType          `json:"type"`
	Props    map[string]any    `json:"props"`
	Scripts  map[string]string `json:"scripts,omitempty"`
	Computed map[string]string `json:"computed,omitempty"`
	ChildIDs []string          `json:"child_ids,omitempty"`
	ParentID string            `json:"parent_id,omitempty"`
}

// Query returns the current state of the requested nodes.
// It returns partial results for nodes that exist and errors for those that don't.
func (t *Tree) Query(ids []string) (map[string]*QueryResult, []error) {
	results := make(map[string]*QueryResult)
	var errs []error

	for _, id := range ids {
		node := t.Find(id)
		if node == nil {
			errs = append(errs, fmt.Errorf("query: node %q not found", id))
			continue
		}
		qr := &QueryResult{
			ID:       node.ID,
			Type:     node.Type,
			Props:    copyMap(node.Props),
			Scripts:  copyStringMap(node.Scripts),
			Computed: copyStringMap(node.Computed),
		}
		for _, child := range node.Children {
			qr.ChildIDs = append(qr.ChildIDs, child.ID)
		}
		if node.parent != nil {
			qr.ParentID = node.parent.ID
		}
		results[id] = qr
	}

	return results, errs
}
