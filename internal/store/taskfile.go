// Package store reads task directories and persists annotation logs as JSONL.
package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/furedea/rater/internal/render"
	"github.com/furedea/rater/internal/task"
)

type taskDoc struct {
	Name   string            `yaml:"name"`
	Layout string            `yaml:"layout"`
	Header []string          `yaml:"header"`
	Fields []fieldDoc        `yaml:"fields"`
	Labels []labelDoc        `yaml:"labels"`
	Keys   map[string]string `yaml:"keys"`
}

type fieldDoc struct {
	Name          string `yaml:"name"`
	Title         string `yaml:"title"`
	Render        string `yaml:"render"`
	Language      string `yaml:"language"`
	LanguageField string `yaml:"language_field"`
}

type labelDoc struct {
	Key  string `yaml:"key"`
	Name string `yaml:"name"`
}

// LoadTask reads and validates <dir>/task.yaml.
func LoadTask(dir string) (task.Task, error) {
	data, err := readInDir(dir, "task.yaml")
	if err != nil {
		return task.Task{}, fmt.Errorf("read task: %w", err)
	}
	t, err := ParseTask(data)
	if err != nil {
		return task.Task{}, fmt.Errorf("parse task %s: %w", filepath.Join(dir, "task.yaml"), err)
	}
	return t, nil
}

// readInDir reads a file confined to dir.
func readInDir(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open dir %s: %w", dir, err)
	}
	data, readErr := root.ReadFile(name)
	closeErr := root.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read %s in %s: %w", name, dir, readErr)
	}
	return data, closeErr
}

// ParseTask decodes task.yaml content into a validated Task.
func ParseTask(data []byte) (task.Task, error) {
	var doc taskDoc
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return task.Task{}, fmt.Errorf("decode yaml: %w", err)
	}
	spec, err := doc.spec()
	if err != nil {
		return task.Task{}, err
	}
	return task.NewTask(spec)
}

func (d taskDoc) spec() (task.Spec, error) {
	layout, err := parseLayout(d.Layout)
	if err != nil {
		return task.Spec{}, err
	}
	fields, err := convertFields(d.Fields)
	if err != nil {
		return task.Spec{}, err
	}
	labels, err := convertLabels(d.Labels)
	if err != nil {
		return task.Spec{}, err
	}
	keys, err := convertKeys(d.Keys)
	if err != nil {
		return task.Spec{}, err
	}
	return task.Spec{Name: d.Name, Layout: layout, Header: d.Header, Fields: fields, Labels: labels, Keys: keys}, nil
}

func parseLayout(s string) (task.Layout, error) {
	if s == "" || s == task.LayoutStacked.String() {
		return task.LayoutStacked, nil
	}
	return 0, fmt.Errorf("%w: unsupported layout %q", task.ErrInvalidTask, s)
}

func convertFields(docs []fieldDoc) ([]task.Field, error) {
	fields := make([]task.Field, 0, len(docs))
	for _, d := range docs {
		f, err := d.field()
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}

func (d fieldDoc) field() (task.Field, error) {
	kind := task.RenderText
	if d.Render != "" {
		parsed, err := task.ParseRender(d.Render)
		if err != nil {
			return task.Field{}, fmt.Errorf("%w: unsupported render %q for field %q", task.ErrInvalidTask, d.Render, d.Name)
		}
		kind = parsed
	}
	if d.Language != "" && !render.HasLanguage(d.Language) {
		return task.Field{}, fmt.Errorf("%w: field %q has unknown language %q", task.ErrInvalidTask, d.Name, d.Language)
	}
	title := d.Title
	if title == "" {
		title = d.Name
	}
	return task.Field{Name: d.Name, Title: title, Render: kind, Language: d.Language, LanguageField: d.LanguageField}, nil
}

func convertKeys(docs map[string]string) (map[task.Binding]string, error) {
	keys := make(map[task.Binding]string, len(docs))
	for name, key := range docs {
		b, err := task.ParseBinding(name)
		if err != nil {
			return nil, fmt.Errorf("%w: keys: %w", task.ErrInvalidTask, err)
		}
		keys[b] = key
	}
	return keys, nil
}

func convertLabels(docs []labelDoc) ([]task.Label, error) {
	labels := make([]task.Label, 0, len(docs))
	for _, d := range docs {
		if utf8.RuneCountInString(d.Key) != 1 {
			return nil, fmt.Errorf("%w: label %q key %q must be one character", task.ErrInvalidTask, d.Name, d.Key)
		}
		key, _ := utf8.DecodeRuneInString(d.Key)
		labels = append(labels, task.Label{Key: key, Name: d.Name})
	}
	return labels, nil
}
