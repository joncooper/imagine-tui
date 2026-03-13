package script

import "fmt"

// Error represents a script execution failure.
type Error struct {
	NodeID    string
	Hook      string // e.g. "on_mount", "on_change", or computed prop name
	Message   string
	IsTimeout bool
}

func (e *Error) Error() string {
	if e.IsTimeout {
		return fmt.Sprintf("script timeout on node %q hook %q: %s", e.NodeID, e.Hook, e.Message)
	}
	return fmt.Sprintf("script error on node %q hook %q: %s", e.NodeID, e.Hook, e.Message)
}
