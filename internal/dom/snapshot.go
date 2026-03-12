package dom

import "fmt"

// SnapshotStore holds named deep copies of tree state.
type SnapshotStore struct {
	snapshots map[string]*Node
}

// NewSnapshotStore creates a new empty snapshot store.
func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{
		snapshots: make(map[string]*Node),
	}
}

// Snapshot deep-copies the current tree state and stores it under the given name.
// If a snapshot with the same name exists, it is overwritten.
func (s *SnapshotStore) Snapshot(name string, tree *Tree) error {
	if tree == nil || tree.Root == nil {
		return fmt.Errorf("snapshot %q: tree is nil", name)
	}
	s.snapshots[name] = tree.Root.deepCopy()
	return nil
}

// Restore replaces the tree's state with the named snapshot.
// The snapshot itself is preserved (a fresh copy is made for the tree).
func (s *SnapshotStore) Restore(name string, tree *Tree) error {
	snap, ok := s.snapshots[name]
	if !ok {
		return fmt.Errorf("restore: snapshot %q not found", name)
	}
	// Make a copy of the snapshot so future restores still work.
	tree.Root = snap.deepCopy()
	tree.index = make(map[string]*Node)
	if err := tree.rebuildIndex(tree.Root); err != nil {
		return fmt.Errorf("restore %q: rebuild index: %w", name, err)
	}
	return nil
}

// List returns the names of all stored snapshots.
func (s *SnapshotStore) List() []string {
	names := make([]string, 0, len(s.snapshots))
	for name := range s.snapshots {
		names = append(names, name)
	}
	return names
}
