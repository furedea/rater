package task

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrInvalidAssignment wraps every validation failure of an assignment.
var ErrInvalidAssignment = errors.New("invalid assignment")

// AssignmentSpec is the unvalidated description of who annotates which items.
// Split deals items out to a group; Items lists items for one annotator each.
// Annotators named in neither see every item.
type AssignmentSpec struct {
	Split *SplitSpec
	Items map[string][]string
}

// SplitSpec deals items round-robin to Annotators so that each item goes to
// Copies of them: item i goes to annotators i, i+1, ..., i+Copies-1 (mod the
// group size). Copies above 1 gives overlapping items for agreement.
type SplitSpec struct {
	Annotators []string
	Copies     int
}

// Assignment limits the items of a workspace each annotator sees.
type Assignment struct {
	ws       Workspace
	assigned map[Annotator]map[string]bool // item ids per assigned annotator
}

// NewAssignment validates spec against ws: annotator names are valid, each
// annotator is assigned at most once, items exist, and nobody assigned ends
// up with no items.
func NewAssignment(ws Workspace, spec AssignmentSpec) (Assignment, error) {
	a := Assignment{ws: ws, assigned: map[Annotator]map[string]bool{}}
	if spec.Split != nil {
		if err := a.addSplit(*spec.Split); err != nil {
			return Assignment{}, fmt.Errorf("%w: split: %w", ErrInvalidAssignment, err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(spec.Items)) {
		if err := a.addList(name, spec.Items[name]); err != nil {
			return Assignment{}, fmt.Errorf("%w: items: %w", ErrInvalidAssignment, err)
		}
	}
	return a, nil
}

// Workspace returns the items who sees, in workspace order: all of them when
// who is not assigned.
func (a Assignment) Workspace(who Annotator) (Workspace, error) {
	ids, ok := a.assigned[who]
	if !ok {
		return a.ws, nil
	}
	items := slices.DeleteFunc(slices.Clone(a.ws.Items), func(it Item) bool { return !ids[it.ID] })
	return NewWorkspace(a.ws.Task, items)
}

// StartSession resumes who on their items. Prior records about items of the
// workspace that who is not assigned stay in the log, where agreement still
// counts them, but are not shown; records about unknown items are rejected.
// Undo acts per item, so leaving out other items' records keeps each shown
// item's history intact.
func (a Assignment) StartSession(who Annotator, prior []Annotation, rec Recorder) (*Session, error) {
	if err := checkRecords(a.ws, prior); err != nil {
		return nil, err
	}
	view, err := a.Workspace(who)
	if err != nil {
		return nil, err
	}
	shown := map[string]bool{}
	for _, it := range view.Items {
		shown[it.ID] = true
	}
	kept := slices.DeleteFunc(slices.Clone(prior), func(r Annotation) bool { return !shown[r.ItemID] })
	return NewSession(view, kept, rec)
}

func (a Assignment) addSplit(s SplitSpec) error {
	group, err := a.newAnnotators(s.Annotators)
	if err != nil {
		return err
	}
	if len(group) == 0 {
		return errors.New("no annotators")
	}
	if s.Copies < 1 || s.Copies > len(group) {
		return fmt.Errorf("copies %d must be between 1 and %d annotators", s.Copies, len(group))
	}
	for _, who := range group {
		a.assigned[who] = map[string]bool{}
	}
	for i, it := range a.ws.Items {
		for k := range s.Copies {
			a.assigned[group[(i+k)%len(group)]][it.ID] = true
		}
	}
	for _, who := range group {
		if len(a.assigned[who]) == 0 {
			return fmt.Errorf("%s gets no items: %d items for %d annotators with %d copies",
				who, len(a.ws.Items), len(group), s.Copies)
		}
	}
	return nil
}

func (a Assignment) addList(name string, ids []string) error {
	group, err := a.newAnnotators([]string{name})
	if err != nil {
		return err
	}
	who := group[0]
	if len(ids) == 0 {
		return fmt.Errorf("%s has an empty item list", who)
	}
	known := map[string]bool{}
	for _, it := range a.ws.Items {
		known[it.ID] = true
	}
	set := map[string]bool{}
	for _, id := range ids {
		if !known[id] {
			return fmt.Errorf("%s: unknown item %q", who, id)
		}
		set[id] = true
	}
	a.assigned[who] = set
	return nil
}

// newAnnotators validates names that are not yet assigned and not repeated.
func (a Assignment) newAnnotators(names []string) ([]Annotator, error) {
	group := make([]Annotator, 0, len(names))
	for _, name := range names {
		who, err := NewAnnotator(name)
		if err != nil {
			return nil, err
		}
		if _, dup := a.assigned[who]; dup || slices.Contains(group, who) {
			return nil, fmt.Errorf("%s is assigned more than once", who)
		}
		group = append(group, who)
	}
	return group, nil
}
