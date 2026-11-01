package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/furedea/rater/internal/task"
)

const maxLineBytes = 16 << 20

// LoadWorkspace reads <dir>/task.yaml and <dir>/items.jsonl and checks them against each other.
func LoadWorkspace(dir string) (task.Workspace, error) {
	t, err := LoadTask(dir)
	if err != nil {
		return task.Workspace{}, err
	}
	path := filepath.Join(dir, "items.jsonl")
	data, err := readInDir(dir, "items.jsonl")
	if err != nil {
		return task.Workspace{}, fmt.Errorf("read items: %w", err)
	}
	items, err := ParseItems(data)
	if err != nil {
		return task.Workspace{}, fmt.Errorf("parse items %s: %w", path, err)
	}
	ws, err := task.NewWorkspace(t, items)
	if err != nil {
		return task.Workspace{}, fmt.Errorf("check items %s: %w", path, err)
	}
	return ws, nil
}

// ParseItems decodes JSONL: one object per line with a string "id" and arbitrary other fields.
// Non-string values are shown as their compact JSON text.
func ParseItems(data []byte) ([]task.Item, error) {
	var items []task.Item
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, maxLineBytes)
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		it, err := parseItem(sc.Bytes())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		items = append(items, it)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan items: %w", err)
	}
	return items, nil
}

func parseItem(line []byte) (task.Item, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return task.Item{}, fmt.Errorf("decode json: %w", err)
	}
	var id string
	if err := json.Unmarshal(raw["id"], &id); err != nil {
		return task.Item{}, fmt.Errorf("id must be a string: %w", err)
	}
	values := make(map[string]string, len(raw))
	for name, msg := range raw {
		values[name] = displayText(msg)
	}
	return task.Item{ID: id, Values: values}, nil
}

func displayText(msg json.RawMessage) string {
	var s string
	if err := json.Unmarshal(msg, &s); err == nil {
		return s
	}
	return string(msg)
}
