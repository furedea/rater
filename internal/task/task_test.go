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

func TestNewTaskSideBySideNeedsTwoFields(t *testing.T) {
	two := []task.Field{{Name: "a"}, {Name: "b"}}
	tests := []struct {
		name    string
		fields  []task.Field
		wantErr bool
	}{
		{"two fields", two, false},
		{"one field", two[:1], true},
		{"three fields", append(append([]task.Field(nil), two...), task.Field{Name: "c"}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			spec.Layout = task.LayoutSideBySide
			spec.Fields = tt.fields

			tk, err := task.NewTask(spec)

			if !tt.wantErr {
				require.NoError(t, err)
				assert.Equal(t, task.LayoutSideBySide, tk.Layout())
				return
			}
			require.ErrorIs(t, err, task.ErrInvalidTask)
			assert.ErrorContains(t, err, "exactly two fields")
		})
	}
}

func TestParseLayout(t *testing.T) {
	for _, l := range []task.Layout{task.LayoutStacked, task.LayoutSideBySide} {
		got, err := task.ParseLayout(l.String())
		require.NoError(t, err)
		assert.Equal(t, l, got)
	}
	_, err := task.ParseLayout("grid")
	assert.Error(t, err)
}

func TestTaskLabelLookup(t *testing.T) {
	tk, err := task.NewTask(validSpec())
	require.NoError(t, err)

	sc, ok := tk.Scheme().(task.SingleChoice)
	require.True(t, ok)

	l, ok := sc.LabelByKey('d')
	assert.True(t, ok)
	assert.Equal(t, "distinct", l.Name)

	_, ok = sc.LabelByKey('x')
	assert.False(t, ok)
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

func TestNewTaskFieldRender(t *testing.T) {
	tests := []struct {
		name    string
		field   task.Field
		wantErr string
	}{
		{"code with fixed language", task.Field{Name: "a", Render: task.RenderCode, Language: "go"}, ""},
		{"code with language from item", task.Field{Name: "a", Render: task.RenderCode, LanguageField: "lang"}, ""},
		{"diff", task.Field{Name: "a", Render: task.RenderDiff}, ""},
		{"markdown", task.Field{Name: "a", Render: task.RenderMarkdown}, ""},
		{"language on non-code field", task.Field{Name: "a", Render: task.RenderDiff, Language: "go"}, "language"},
		{"language field on non-code field", task.Field{Name: "a", LanguageField: "lang"}, "language"},
		{"language field is the field itself", task.Field{Name: "a", Render: task.RenderCode, LanguageField: "a"}, "language"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			spec.Fields = []task.Field{tt.field}
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

func TestParseRender(t *testing.T) {
	for _, r := range []task.Render{task.RenderText, task.RenderDiff, task.RenderCode, task.RenderMarkdown} {
		got, err := task.ParseRender(r.String())
		require.NoError(t, err)
		assert.Equal(t, r, got)
	}
	_, err := task.ParseRender("html")
	assert.Error(t, err)
}

func TestTaskKeysDefaults(t *testing.T) {
	tk, err := task.NewTask(validSpec())
	require.NoError(t, err)

	for key, want := range map[string]task.Binding{
		"up": task.BindingScrollUp, "down": task.BindingScrollDown,
		"pgup": task.BindingPageUp, "pgdown": task.BindingPageDown,
		"]": task.BindingNextHunk, "[": task.BindingPrevHunk,
	} {
		got, ok := tk.BindingFor(key)
		assert.True(t, ok, key)
		assert.Equal(t, want, got, key)
	}
	_, ok := tk.BindingFor("s")
	assert.False(t, ok)
}

func TestTaskKeysOverride(t *testing.T) {
	spec := validSpec()
	spec.Keys = map[task.Binding]string{task.BindingNextHunk: "ctrl+j"}
	tk, err := task.NewTask(spec)
	require.NoError(t, err)

	b, ok := tk.BindingFor("ctrl+j")
	assert.True(t, ok)
	assert.Equal(t, task.BindingNextHunk, b)
	assert.Equal(t, "ctrl+j", tk.KeyFor(task.BindingNextHunk))
	_, ok = tk.BindingFor("]")
	assert.False(t, ok)
}

func TestNewTaskKeyCollisions(t *testing.T) {
	tests := []struct {
		name    string
		keys    map[task.Binding]string
		labels  []task.Label
		wantErr string
	}{
		{"binding equals label key", map[task.Binding]string{task.BindingNextHunk: "s"}, nil, `collides with label "same"`},
		{"default binding equals label key", nil, []task.Label{{Key: ']', Name: "close"}}, `collides with label "close"`},
		{"binding equals control key", map[task.Binding]string{task.BindingScrollDown: "tab"}, nil, "reserved"},
		{"two bindings share a key", map[task.Binding]string{task.BindingScrollDown: "j", task.BindingPageDown: "j"}, nil, "bound twice"},
		{"empty binding", map[task.Binding]string{task.BindingScrollUp: ""}, nil, "unusable key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			spec.Keys = tt.keys
			if tt.labels != nil {
				spec.Labels = tt.labels
			}
			_, err := task.NewTask(spec)
			require.ErrorIs(t, err, task.ErrInvalidTask)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestParseBinding(t *testing.T) {
	for _, b := range task.Bindings() {
		got, err := task.ParseBinding(b.String())
		require.NoError(t, err)
		assert.Equal(t, b, got)
	}
	_, err := task.ParseBinding("jump")
	assert.Error(t, err)
}
