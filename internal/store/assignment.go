package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/furedea/rater/internal/task"
)

// assignmentFile is optional; without it every annotator sees every item.
const assignmentFile = "assignment.yaml"

type assignmentDoc struct {
	Split *splitDoc           `yaml:"split"`
	Items map[string][]string `yaml:"items"`
}

type splitDoc struct {
	Annotators []string `yaml:"annotators"`
	Copies     int      `yaml:"copies"`
}

// LoadAssignment reads <dir>/assignment.yaml and validates it against ws.
// A task without the file has an empty assignment.
func LoadAssignment(dir string, ws task.Workspace) (task.Assignment, error) {
	path := filepath.Join(dir, assignmentFile)
	data, err := readInDir(dir, assignmentFile)
	if errors.Is(err, fs.ErrNotExist) {
		return task.NewAssignment(ws, task.AssignmentSpec{})
	}
	if err != nil {
		return task.Assignment{}, fmt.Errorf("read assignment: %w", err)
	}
	spec, err := parseAssignment(data)
	if err != nil {
		return task.Assignment{}, fmt.Errorf("parse assignment %s: %w", path, err)
	}
	plan, err := task.NewAssignment(ws, spec)
	if err != nil {
		return task.Assignment{}, fmt.Errorf("check assignment %s: %w", path, err)
	}
	return plan, nil
}

func parseAssignment(data []byte) (task.AssignmentSpec, error) {
	var doc assignmentDoc
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return task.AssignmentSpec{}, fmt.Errorf("decode yaml: %w", err)
	}
	spec := task.AssignmentSpec{Items: doc.Items}
	if doc.Split != nil {
		spec.Split = &task.SplitSpec{Annotators: doc.Split.Annotators, Copies: doc.Split.Copies}
	}
	return spec, nil
}
