package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

func checklistSpec() task.Spec {
	return task.Spec{
		Name:   "criteria",
		Fields: []task.Field{{Name: "sentence", Title: "Sentence"}},
		Checklist: &task.ChecklistSpec{
			Criteria: []task.Criterion{
				{Name: "specific", Title: "Names a concrete construct"},
				{Name: "checkable", Title: "Can be checked in a diff"},
				{Name: "scoped", Title: "States where it applies"},
			},
			Answers: []task.Label{{Key: 'y', Name: "yes"}, {Key: 'n', Name: "no"}, {Key: '?', Name: "unsure"}},
			Rule:    task.AllPass{Pass: "yes", Fail: "no"},
		},
	}
}

func checklistOf(t *testing.T, tk task.Task) task.Checklist {
	t.Helper()
	c, ok := tk.Scheme().(task.Checklist)
	require.True(t, ok, "scheme is %T", tk.Scheme())
	return c
}

func TestNewTaskChecklist(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*task.Spec)
		wantErr string
	}{
		{"valid", func(*task.Spec) {}, ""},
		{"labels and checklist", func(s *task.Spec) { s.Labels = []task.Label{{Key: 's', Name: "same"}} }, "not both"},
		{"no criteria", func(s *task.Spec) { s.Checklist.Criteria = nil }, "no criteria"},
		{"empty criterion name", func(s *task.Spec) { s.Checklist.Criteria[0].Name = "" }, "criterion name is empty"},
		{"duplicate criterion", func(s *task.Spec) { s.Checklist.Criteria[1].Name = "specific" }, `duplicate criterion "specific"`},
		{"no answers", func(s *task.Spec) { s.Checklist.Answers = nil }, "no answers"},
		{"duplicate answer key", func(s *task.Spec) { s.Checklist.Answers[1].Key = 'y' }, "duplicate answer key"},
		{"pass is not an answer", func(s *task.Spec) { s.Checklist.Rule.Pass = "ok" }, `pass answer "ok"`},
		{"fail is not an answer", func(s *task.Spec) { s.Checklist.Rule.Fail = "bad" }, `fail answer "bad"`},
		{"pass equals fail", func(s *task.Spec) { s.Checklist.Rule.Fail = "yes" }, "differ"},
		{"answer key collides with a binding", func(s *task.Spec) { s.Checklist.Answers[2].Key = ']' }, `collides with answer "unsure"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := checklistSpec()
			cl := *spec.Checklist
			cl.Criteria = append([]task.Criterion(nil), cl.Criteria...)
			cl.Answers = append([]task.Label(nil), cl.Answers...)
			spec.Checklist = &cl
			tt.mutate(&spec)

			tk, err := task.NewTask(spec)

			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Len(t, checklistOf(t, tk).Criteria(), 3)
				return
			}
			require.ErrorIs(t, err, task.ErrInvalidTask)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNewTaskWithLabelsIsSingleChoice(t *testing.T) {
	tk, err := task.NewTask(validSpec())
	require.NoError(t, err)

	sc, ok := tk.Scheme().(task.SingleChoice)

	require.True(t, ok)
	assert.Equal(t, validSpec().Labels, sc.Labels())
}

func TestAllPassVerdict(t *testing.T) {
	tk, err := task.NewTask(checklistSpec())
	require.NoError(t, err)
	cl := checklistOf(t, tk)
	tests := []struct {
		name    string
		answers map[string]string
		want    task.Verdict
	}{
		{"nothing answered", nil, task.VerdictUndecided},
		{"all pass", map[string]string{"specific": "yes", "checkable": "yes", "scoped": "yes"}, task.VerdictPass},
		{"one fail", map[string]string{"specific": "yes", "checkable": "no", "scoped": "yes"}, task.VerdictFail},
		{"a fail decides before the rest is answered", map[string]string{"checkable": "no"}, task.VerdictFail},
		{"a fail outweighs an unsure", map[string]string{"specific": "unsure", "checkable": "no", "scoped": "yes"}, task.VerdictFail},
		{"one unsure", map[string]string{"specific": "yes", "checkable": "unsure", "scoped": "yes"}, task.VerdictUndecided},
		{"one unanswered", map[string]string{"specific": "yes", "checkable": "yes"}, task.VerdictUndecided},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cl.Verdict(tt.answers))
		})
	}
}

func TestParseVerdictRule(t *testing.T) {
	got, err := task.ParseVerdictRule("all_pass")
	require.NoError(t, err)
	assert.Equal(t, "all_pass", got.String())

	_, err = task.ParseVerdictRule("majority")
	assert.Error(t, err)
}
