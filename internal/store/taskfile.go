// Package store reads task directories and persists annotation logs as JSONL.
package store

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/furedea/rater/internal/render"
	"github.com/furedea/rater/internal/task"
)

type taskDoc struct {
	Name      string            `yaml:"name"`
	Layout    string            `yaml:"layout"`
	Header    []string          `yaml:"header"`
	Fields    []fieldDoc        `yaml:"fields"`
	Labels    []labelDoc        `yaml:"labels"`
	Checklist *checklistDoc     `yaml:"checklist"`
	Keys      map[string]string `yaml:"keys"`
}

type checklistDoc struct {
	Criteria []criterionDoc `yaml:"criteria"`
	Answers  []labelDoc     `yaml:"answers"`
	Verdict  verdictDoc     `yaml:"verdict"`
}

type criterionDoc struct {
	Name  string `yaml:"name"`
	Title string `yaml:"title"`
}

type verdictDoc struct {
	Rule string `yaml:"rule"`
	Pass string `yaml:"pass"`
	Fail string `yaml:"fail"`
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
	checklist, err := convertChecklist(d.Checklist)
	if err != nil {
		return task.Spec{}, err
	}
	keys, err := convertKeys(d.Keys)
	if err != nil {
		return task.Spec{}, err
	}
	return task.Spec{
		Name: d.Name, Layout: layout, Header: d.Header, Fields: fields,
		Labels: labels, Checklist: checklist, Keys: keys,
	}, nil
}

// convertChecklist converts the optional checklist section; without one the task is single-choice.
func convertChecklist(d *checklistDoc) (*task.ChecklistSpec, error) {
	if d == nil {
		return nil, nil
	}
	spec, err := d.spec()
	if err != nil {
		return nil, err
	}
	return &spec, nil
}

func (d checklistDoc) spec() (task.ChecklistSpec, error) {
	answers, err := convertLabels(d.Answers)
	if err != nil {
		return task.ChecklistSpec{}, err
	}
	rule, err := d.Verdict.rule()
	if err != nil {
		return task.ChecklistSpec{}, err
	}
	criteria := make([]task.Criterion, 0, len(d.Criteria))
	for _, c := range d.Criteria {
		criteria = append(criteria, task.Criterion{Name: c.Name, Title: cmp.Or(c.Title, c.Name)})
	}
	return task.ChecklistSpec{Criteria: criteria, Answers: answers, Rule: rule}, nil
}

func (d verdictDoc) rule() (task.AllPass, error) {
	if d.Rule == "" {
		return task.AllPass{}, fmt.Errorf("%w: checklist verdict rule is required (rule: %s)", task.ErrInvalidTask, task.RuleAllPass)
	}
	kind, err := task.ParseVerdictRule(d.Rule)
	if err != nil {
		return task.AllPass{}, fmt.Errorf("%w: checklist verdict: %w", task.ErrInvalidTask, err)
	}
	switch kind {
	case task.RuleAllPass:
		return task.AllPass{Pass: d.Pass, Fail: d.Fail}, nil
	}
	return task.AllPass{}, fmt.Errorf("%w: checklist verdict rule %s is not supported", task.ErrInvalidTask, kind)
}

func parseLayout(s string) (task.Layout, error) {
	if s == "" {
		return task.LayoutStacked, nil
	}
	l, err := task.ParseLayout(s)
	if err != nil {
		return 0, fmt.Errorf("%w: unsupported layout %q", task.ErrInvalidTask, s)
	}
	return l, nil
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
		key, err := parseLabelKey(d.Key)
		if err != nil {
			return nil, fmt.Errorf("%w: label %q: %w", task.ErrInvalidTask, d.Name, err)
		}
		labels = append(labels, task.Label{Key: key, Name: d.Name})
	}
	return labels, nil
}

// parseLabelKey accepts any spelling of a key that types one character, such as "S" or "shift+s".
func parseLabelKey(written string) (rune, error) {
	key, err := task.ParseKey(written)
	if err != nil {
		return 0, fmt.Errorf("key %q must be one character: %w", written, err)
	}
	r, ok := key.Rune()
	if !ok {
		return 0, fmt.Errorf("key %q must be one character", written)
	}
	return r, nil
}
