package agreement

import (
	"fmt"

	"github.com/furedea/rater/internal/task"
)

// Outcome is how the annotators' completed annotations of one item compare.
type Outcome int

// Outcomes. As in Report, only labeled states take part: held and pending
// states, including an incomplete checklist, count as missing.
const (
	// OutcomeUncompared: fewer than two annotators completed the item.
	OutcomeUncompared Outcome = iota
	// OutcomeAgreed: every completed annotation gives the same answers.
	OutcomeAgreed
	// OutcomeDisagreed: two completed annotations differ in the label or,
	// for a checklist, in the answer to some criterion.
	OutcomeDisagreed
)

func (o Outcome) String() string {
	switch o {
	case OutcomeUncompared:
		return "uncompared"
	case OutcomeAgreed:
		return "agreed"
	case OutcomeDisagreed:
		return "disagreed"
	}
	return fmt.Sprintf("Outcome(%d)", int(o))
}

// Compare compares one item's states, one per annotator, under scheme. A
// checklist is compared per criterion rather than by verdict, because the
// adjudicated result is the answers and the verdict follows from them.
func Compare(scheme task.Scheme, states []task.State) Outcome {
	outcome := OutcomeAgreed
	for _, valueOf := range answerValues(scheme) {
		unit := make([]Value, 0, len(states))
		for _, s := range states {
			unit = append(unit, labeledValue(s, valueOf))
		}
		present := presentValues(unit)
		if len(present) < pair {
			return OutcomeUncompared
		}
		if !allEqual(present) {
			outcome = OutcomeDisagreed
		}
	}
	return outcome
}

// answerValues reads each answer a scheme records: the label, or one answer per criterion.
func answerValues(scheme task.Scheme) []func(task.State) string {
	switch s := scheme.(type) {
	case task.SingleChoice:
		return []func(task.State) string{labelValue}
	case task.Checklist:
		var values []func(task.State) string
		for _, cr := range s.Criteria() {
			values = append(values, criterionValue(cr.Name))
		}
		return values
	}
	return nil
}

func labelValue(s task.State) string { return s.Label }

func criterionValue(name string) func(task.State) string {
	return func(s task.State) string { return s.Answers[name] }
}

func allEqual(cs []string) bool {
	for _, c := range cs[1:] {
		if c != cs[0] {
			return false
		}
	}
	return true
}
