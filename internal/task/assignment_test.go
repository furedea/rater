package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

func TestAssignmentSelectsItemsPerAnnotator(t *testing.T) {
	split := &task.SplitSpec{Annotators: []string{"alice", "bob", "carol"}, Copies: 2}
	tests := []struct {
		name string
		spec task.AssignmentSpec
		who  string
		want []string
	}{
		{"no assignment shows every item", task.AssignmentSpec{}, "alice", []string{"1", "2", "3", "4"}},
		{"unlisted annotator sees every item", task.AssignmentSpec{Items: map[string][]string{"dave": {"2"}}}, "erin", []string{"1", "2", "3", "4"}},
		{"explicit list keeps item order", task.AssignmentSpec{Items: map[string][]string{"dave": {"3", "1"}}}, "dave", []string{"1", "3"}},
		{"split round-robin with overlap, first annotator", task.AssignmentSpec{Split: split}, "alice", []string{"1", "3", "4"}},
		{"split round-robin with overlap, second annotator", task.AssignmentSpec{Split: split}, "bob", []string{"1", "2", "4"}},
		{"split round-robin with overlap, third annotator", task.AssignmentSpec{Split: split}, "carol", []string{"2", "3"}},
		{"split without overlap partitions items", task.AssignmentSpec{Split: &task.SplitSpec{Annotators: []string{"alice", "bob"}, Copies: 1}}, "bob", []string{"2", "4"}},
		{"split and explicit lists combine", task.AssignmentSpec{Split: split, Items: map[string][]string{"dave": {"4"}}}, "dave", []string{"4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan, err := task.NewAssignment(newWorkspace(t, "1", "2", "3", "4"), tt.spec)
			require.NoError(t, err)

			view, err := plan.Workspace(annotator(t, tt.who))
			require.NoError(t, err)

			assert.Equal(t, tt.want, itemIDs(view))
		})
	}
}

func TestNewAssignmentRejectsInvalidSpecs(t *testing.T) {
	tests := []struct {
		name string
		spec task.AssignmentSpec
	}{
		{"unknown item", task.AssignmentSpec{Items: map[string][]string{"dave": {"9"}}}},
		{"empty item list", task.AssignmentSpec{Items: map[string][]string{"dave": {}}}},
		{"invalid annotator name", task.AssignmentSpec{Items: map[string][]string{"Dave": {"1"}}}},
		{"no split annotators", task.AssignmentSpec{Split: &task.SplitSpec{Copies: 1}}},
		{"invalid split annotator", task.AssignmentSpec{Split: &task.SplitSpec{Annotators: []string{"a b"}, Copies: 1}}},
		{"duplicate split annotator", task.AssignmentSpec{Split: &task.SplitSpec{Annotators: []string{"alice", "alice"}, Copies: 1}}},
		{"zero copies", task.AssignmentSpec{Split: &task.SplitSpec{Annotators: []string{"alice"}, Copies: 0}}},
		{"more copies than annotators", task.AssignmentSpec{Split: &task.SplitSpec{Annotators: []string{"alice", "bob"}, Copies: 3}}},
		{"split leaves an annotator without items", task.AssignmentSpec{Split: &task.SplitSpec{Annotators: []string{"a", "b", "c", "d", "e"}, Copies: 1}}},
		{"annotator both split and listed", task.AssignmentSpec{
			Split: &task.SplitSpec{Annotators: []string{"alice", "bob"}, Copies: 1},
			Items: map[string][]string{"alice": {"1"}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := task.NewAssignment(newWorkspace(t, "1", "2", "3", "4"), tt.spec)
			assert.ErrorIs(t, err, task.ErrInvalidAssignment)
		})
	}
}

func TestAssignedSessionResumesOnlyAssignedItems(t *testing.T) {
	plan, err := task.NewAssignment(newWorkspace(t, "1", "2", "3"),
		task.AssignmentSpec{Items: map[string][]string{"dave": {"2", "3"}}})
	require.NoError(t, err)
	prior := []task.Annotation{
		{ItemID: "1", Action: task.ActionLabel, Label: "same"},
		{ItemID: "2", Action: task.ActionLabel, Label: "distinct"},
	}

	s, err := plan.StartSession(annotator(t, "dave"), prior, &fakeRecorder{})
	require.NoError(t, err)

	n, total := s.Position()
	assert.Equal(t, []int{2, 2}, []int{n, total}, "resumes at item 3, the first pending of two")
	assert.Equal(t, "3", s.Current().ID)
	assert.Equal(t, 1, s.Labeled())
	assert.Equal(t, "distinct", s.State("2").Label)
}

func TestAssignedSessionStillRejectsRecordsAboutUnknownItems(t *testing.T) {
	plan, err := task.NewAssignment(newWorkspace(t, "1", "2"),
		task.AssignmentSpec{Items: map[string][]string{"dave": {"2"}}})
	require.NoError(t, err)

	_, err = plan.StartSession(annotator(t, "dave"), []task.Annotation{{ItemID: "9", Action: task.ActionHold}}, &fakeRecorder{})

	assert.ErrorIs(t, err, task.ErrUnknownItem)
}

func annotator(t *testing.T, name string) task.Annotator {
	t.Helper()
	who, err := task.NewAnnotator(name)
	require.NoError(t, err)
	return who
}

func itemIDs(ws task.Workspace) []string {
	ids := make([]string, 0, len(ws.Items))
	for _, it := range ws.Items {
		ids = append(ids, it.ID)
	}
	return ids
}
