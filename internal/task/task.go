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

// Layouts. Side-by-side shows exactly two fields in two columns.
const (
	LayoutStacked Layout = iota
	LayoutSideBySide
)

func (l Layout) String() string {
	switch l {
	case LayoutStacked:
		return "stacked"
	case LayoutSideBySide:
		return "side-by-side"
	}
	return fmt.Sprintf("Layout(%d)", int(l))
}

// ParseLayout is the inverse of Layout.String.
func ParseLayout(s string) (Layout, error) {
	for _, l := range []Layout{LayoutStacked, LayoutSideBySide} {
		if l.String() == s {
			return l, nil
		}
	}
	return 0, fmt.Errorf("unknown layout %q", s)
}

// Render says how a field's text is displayed.
type Render int

// Renders.
const (
	RenderText Render = iota
	RenderDiff
	RenderCode
	RenderMarkdown
)

func (r Render) String() string {
	switch r {
	case RenderText:
		return "text"
	case RenderDiff:
		return "diff"
	case RenderCode:
		return "code"
	case RenderMarkdown:
		return "markdown"
	}
	return fmt.Sprintf("Render(%d)", int(r))
}

// ParseRender is the inverse of Render.String.
func ParseRender(s string) (Render, error) {
	for _, r := range []Render{RenderText, RenderDiff, RenderCode, RenderMarkdown} {
		if r.String() == s {
			return r, nil
		}
	}
	return 0, fmt.Errorf("unknown render %q", s)
}

// Field is one item field shown to the annotator. Language and LanguageField
// apply to code only: the item's LanguageField value, when present, overrides
// Language.
type Field struct {
	Name          string
	Title         string
	Render        Render
	Language      string
	LanguageField string
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
	scheme Scheme
	keys   Keys
}

// Spec is the unvalidated input from which a Task is built.
type Spec struct {
	Name   string
	Layout Layout
	Header []string
	Fields []Field
	// Labels makes a single-choice task; Checklist makes a checklist task.
	// Exactly one of them is set.
	Labels    []Label
	Checklist *ChecklistSpec
	// Keys overrides the default key of each listed binding.
	Keys map[Binding]string
}

const sideBySidePanes = 2

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
	if spec.Layout == LayoutSideBySide && len(spec.Fields) != sideBySidePanes {
		return Task{}, invalidTask("layout %s needs exactly two fields, got %d", spec.Layout, len(spec.Fields))
	}
	scheme, err := newScheme(spec.Labels, spec.Checklist)
	if err != nil {
		return Task{}, err
	}
	keys, err := newKeys(spec.Keys, scheme)
	if err != nil {
		return Task{}, err
	}
	return Task{
		name:   spec.Name,
		layout: spec.Layout,
		header: append([]string(nil), spec.Header...),
		fields: append([]Field(nil), spec.Fields...),
		scheme: scheme,
		keys:   keys,
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

// Scheme returns how items are answered.
func (t Task) Scheme() Scheme { return t.scheme }

// BindingFor finds the view binding triggered by key.
func (t Task) BindingFor(key string) (Binding, bool) { return t.keys.bindingFor(key) }

// KeyFor returns the key that triggers b.
func (t Task) KeyFor(b Binding) string { return t.keys[b].String() }

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
		if err := validateLanguage(f); err != nil {
			return err
		}
	}
	return nil
}

func validateLanguage(f Field) error {
	if f.Render != RenderCode && (f.Language != "" || f.LanguageField != "") {
		return invalidTask("field %q sets a language but renders as %s", f.Name, f.Render)
	}
	if f.LanguageField == f.Name {
		return invalidTask("field %q takes its language from itself", f.Name)
	}
	return nil
}

// validateLabels checks keyed labels; noun ("label" or "answer") names them in messages.
func validateLabels(labels []Label, noun string) error {
	if len(labels) == 0 {
		return invalidTask("no %ss", noun)
	}
	keys := map[rune]bool{}
	names := map[string]bool{}
	for _, l := range labels {
		if err := validateLabel(l, noun); err != nil {
			return err
		}
		if keys[l.Key] {
			return invalidTask("duplicate %s key %q", noun, l.Key)
		}
		if names[l.Name] {
			return invalidTask("duplicate %s name %q", noun, l.Name)
		}
		keys[l.Key] = true
		names[l.Name] = true
	}
	return nil
}

func validateLabel(l Label, noun string) error {
	if l.Name == "" {
		return invalidTask("%s name is empty", noun)
	}
	if !unicode.IsPrint(l.Key) || unicode.IsSpace(l.Key) {
		return invalidTask("%s %q has unusable key %q", noun, l.Name, l.Key)
	}
	return nil
}

func invalidTask(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidTask, fmt.Sprintf(format, args...))
}
