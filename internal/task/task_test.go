package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

func validSpec() task.Spec {
	return task.Spec{
		Name:   "pairs",
		Header: []string{"repo"},
		Fields: []task.Field{{Name: "a", Title: "A"}, {Name: "b", Title: "B"}},
		Labels: []task.Label{{Key: 's', Name: "same"}, {Key: 'd', Name: "distinct"}},
	}
}

func TestNewTask(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*task.Spec)
		wantErr string
	}{
		{"valid", func(*task.Spec) {}, ""},
		{"empty name", func(s *task.Spec) { s.Name = " " }, "name is empty"},
		{"no fields", func(s *task.Spec) { s.Fields = nil }, "no fields"},
		{"empty field name", func(s *task.Spec) { s.Fields[0].Name = "" }, "field name is empty"},
		{"duplicate field", func(s *task.Spec) { s.Fields[1].Name = "a" }, `duplicate field "a"`},
		{"no labels", func(s *task.Spec) { s.Labels = nil }, "no labels"},
		{"empty label name", func(s *task.Spec) { s.Labels[0].Name = "" }, "label name is empty"},
		{"duplicate key", func(s *task.Spec) { s.Labels[1].Key = 's' }, "duplicate label key"},
		{"duplicate label name", func(s *task.Spec) { s.Labels[1].Name = "same" }, "duplicate label name"},
		{"space key", func(s *task.Spec) { s.Labels[0].Key = ' ' }, "unusable key"},
		{"control key", func(s *task.Spec) { s.Labels[0].Key = '\t' }, "unusable key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			spec.Fields = append([]task.Field(nil), spec.Fields...)
			spec.Labels = append([]task.Label(nil), spec.Labels...)
			tt.mutate(&spec)
			_, err := task.NewTask(spec)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, task.ErrInvalidTask)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestTaskLabelLookup(t *testing.T) {
	tk, err := task.NewTask(validSpec())
	require.NoError(t, err)

	l, ok := tk.LabelByKey('d')
	assert.True(t, ok)
	assert.Equal(t, "distinct", l.Name)

	_, ok = tk.LabelByKey('x')
	assert.False(t, ok)
	assert.True(t, tk.HasLabel("same"))
	assert.False(t, tk.HasLabel("other"))
}

func TestNewWorkspace(t *testing.T) {
	tk, err := task.NewTask(validSpec())
	require.NoError(t, err)
	item := func(id string, kv ...string) task.Item {
		values := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			values[kv[i]] = kv[i+1]
		}
		return task.Item{ID: id, Values: values}
	}
	tests := []struct {
		name    string
		items   []task.Item
		wantErr string
	}{
		{"valid", []task.Item{item("1", "a", "x", "b", "y")}, ""},
		{"no items", nil, "no items"},
		{"empty id", []task.Item{item("", "a", "x", "b", "y")}, "empty id"},
		{"duplicate id", []task.Item{item("1", "a", "x", "b", "y"), item("1", "a", "x", "b", "y")}, "duplicate id"},
		{"missing field", []task.Item{item("1", "a", "x")}, `lacks field "b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := task.NewWorkspace(tk, tt.items)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, task.ErrInvalidItems)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
