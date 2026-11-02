package task

import (
	"errors"
	"fmt"
)

// Item is one thing to annotate: an id plus display values keyed by field name.
type Item struct {
	ID     string
	Values map[string]string
}

// ErrInvalidItems wraps every validation failure of an item list.
var ErrInvalidItems = errors.New("invalid items")

// Workspace is a task together with the items it applies to.
type Workspace struct {
	Task  Task
	Items []Item
}

// NewWorkspace checks that items have unique ids and carry every field the task shows.
func NewWorkspace(t Task, items []Item) (Workspace, error) {
	if len(items) == 0 {
		return Workspace{}, fmt.Errorf("%w: no items", ErrInvalidItems)
	}
	seen := map[string]bool{}
	for i, it := range items {
		if err := checkItem(t, it, seen); err != nil {
			return Workspace{}, fmt.Errorf("item %d: %w", i+1, err)
		}
	}
	return Workspace{Task: t, Items: append([]Item(nil), items...)}, nil
}

func checkItem(t Task, it Item, seen map[string]bool) error {
	if it.ID == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidItems)
	}
	if seen[it.ID] {
		return fmt.Errorf("%w: duplicate id %q", ErrInvalidItems, it.ID)
	}
	seen[it.ID] = true
	for _, f := range t.Fields() {
		if _, ok := it.Values[f.Name]; !ok {
			return fmt.Errorf("%w: item %q lacks field %q", ErrInvalidItems, it.ID, f.Name)
		}
	}
	return nil
}
