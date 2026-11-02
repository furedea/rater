// Package task holds the pure annotation domain: task definitions, items,
// annotation records, and the session that turns user intent into records.
package task

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Layout says how an item's fields are arranged on screen.
type Layout int

// Layouts. Later milestones add side-by-side.
const (
	LayoutStacked Layout = iota
)

func (l Layout) String() string {
	switch l {
	case LayoutStacked:
		return "stacked"
	}
	return fmt.Sprintf("Layout(%d)", int(l))
}

// Render says how a field's text is displayed. Later milestones add diff and code.
type Render int

// Renders.
const (
	RenderText Render = iota
)

func (r Render) String() string {
	switch r {
	case RenderText:
		return "text"
	}
	return fmt.Sprintf("Render(%d)", int(r))
}

// Field is one item field shown to the annotator.
type Field struct {
	Name   string
	Title  string
	Render Render
}

// Label is one single-choice answer bound to a key.
type Label struct {
	Key  rune
	Name string
}

// Task is a validated task definition.
type Task struct {
	name   string
	layout Layout
	header []string
	fields []Field
	labels []Label
}

// Spec is the unvalidated input from which a Task is built.
type Spec struct {
	Name   string
	Layout Layout
	Header []string
	Fields []Field
	Labels []Label
}

// ErrInvalidTask wraps every validation failure of a task definition.
var ErrInvalidTask = errors.New("invalid task")

// NewTask validates spec once; the returned Task needs no further checks.
func NewTask(spec Spec) (Task, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return Task{}, invalidTask("name is empty")
	}
	if err := validateFields(spec.Fields); err != nil {
		return Task{}, err
	}
	if err := validateLabels(spec.Labels); err != nil {
		return Task{}, err
	}
	return Task{
		name:   spec.Name,
		layout: spec.Layout,
		header: append([]string(nil), spec.Header...),
		fields: append([]Field(nil), spec.Fields...),
		labels: append([]Label(nil), spec.Labels...),
	}, nil
}

// Name returns the task name.
func (t Task) Name() string { return t.name }

// Layout returns the field arrangement.
func (t Task) Layout() Layout { return t.layout }

// Header returns the names of the item fields shown as metadata.
func (t Task) Header() []string { return append([]string(nil), t.header...) }

// Fields returns the item fields shown to the annotator.
func (t Task) Fields() []Field { return append([]Field(nil), t.fields...) }

// Labels returns the single-choice labels in configured order.
func (t Task) Labels() []Label { return append([]Label(nil), t.labels...) }

// LabelByKey finds the label bound to key.
func (t Task) LabelByKey(key rune) (Label, bool) {
	for _, l := range t.labels {
		if l.Key == key {
			return l, true
		}
	}
	return Label{}, false
}

// HasLabel reports whether name is a configured label name.
func (t Task) HasLabel(name string) bool {
	for _, l := range t.labels {
		if l.Name == name {
			return true
		}
	}
	return false
}

func validateFields(fields []Field) error {
	if len(fields) == 0 {
		return invalidTask("no fields")
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if f.Name == "" {
			return invalidTask("field name is empty")
		}
		if seen[f.Name] {
			return invalidTask("duplicate field %q", f.Name)
		}
		seen[f.Name] = true
	}
	return nil
}

func validateLabels(labels []Label) error {
	if len(labels) == 0 {
		return invalidTask("no labels")
	}
	keys := map[rune]bool{}
	names := map[string]bool{}
	for _, l := range labels {
		if err := validateLabel(l); err != nil {
			return err
		}
		if keys[l.Key] {
			return invalidTask("duplicate label key %q", l.Key)
		}
		if names[l.Name] {
			return invalidTask("duplicate label name %q", l.Name)
		}
		keys[l.Key] = true
		names[l.Name] = true
	}
	return nil
}

func validateLabel(l Label) error {
	if l.Name == "" {
		return invalidTask("label name is empty")
	}
	if !unicode.IsPrint(l.Key) || unicode.IsSpace(l.Key) {
		return invalidTask("label %q has unusable key %q", l.Name, l.Key)
	}
	return nil
}

func invalidTask(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidTask, fmt.Sprintf(format, args...))
}
