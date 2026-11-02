package agreement_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/furedea/rater/internal/agreement"
	"github.com/furedea/rater/internal/task"
)

func labeled(l string) task.State { return task.State{Status: task.StatusLabeled, Label: l} }

func complete(answers map[string]string) task.State {
	return task.State{Status: task.StatusLabeled, Answers: answers}
}

func TestCompareSingleChoiceItem(t *testing.T) {
	scheme := mustScheme(t, singleChoiceSpec())
	tests := []struct {
		name   string
		states []task.State
		want   agreement.Outcome
	}{
		{"same labels agree", []task.State{labeled("same"), labeled("same")}, agreement.OutcomeAgreed},
		{"different labels disagree", []task.State{labeled("same"), labeled("distinct")}, agreement.OutcomeDisagreed},
		{"one labeled is not compared", []task.State{labeled("same"), {}}, agreement.OutcomeUncompared},
		{
			"held label is left out like in agreement",
			[]task.State{labeled("same"), {Status: task.StatusHeld}},
			agreement.OutcomeUncompared,
		},
		{
			"two of three agree while the third holds",
			[]task.State{labeled("same"), {Status: task.StatusHeld}, labeled("same")},
			agreement.OutcomeAgreed,
		},
		{
			"any differing pair among three disagrees",
			[]task.State{labeled("same"), labeled("same"), labeled("distinct")},
			agreement.OutcomeDisagreed,
		},
		{"nobody labeled is not compared", []task.State{{}, {}}, agreement.OutcomeUncompared},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, agreement.Compare(scheme, tt.states))
		})
	}
}

func TestCompareChecklistItem(t *testing.T) {
	scheme := mustScheme(t, checklistSpec())
	yesYes := map[string]string{"concrete": "yes", "scoped": "yes"}
	yesNo := map[string]string{"concrete": "yes", "scoped": "no"}
	noNo := map[string]string{"concrete": "no", "scoped": "no"}
	tests := []struct {
		name   string
		states []task.State
		want   agreement.Outcome
	}{
		{"same answers agree", []task.State{complete(yesYes), complete(yesYes)}, agreement.OutcomeAgreed},
		{"one differing criterion disagrees", []task.State{complete(yesYes), complete(yesNo)}, agreement.OutcomeDisagreed},
		{
			"same verdict with different answers still disagrees",
			[]task.State{complete(yesNo), complete(noNo)},
			agreement.OutcomeDisagreed,
		},
		{
			"incomplete checklist is left out",
			[]task.State{complete(yesYes), {Status: task.StatusPending, Answers: map[string]string{"concrete": "no"}}},
			agreement.OutcomeUncompared,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, agreement.Compare(scheme, tt.states))
		})
	}
}

func mustScheme(t *testing.T, spec task.Spec) task.Scheme {
	t.Helper()
	return workspace(t, spec, "1").Task.Scheme()
}
