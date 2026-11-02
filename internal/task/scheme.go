package task

import "fmt"

// Scheme is how an annotator answers an item: with one label, or with one
// answer per criterion.
//
//sumtype:decl
type Scheme interface{ isScheme() }

// SingleChoice answers an item with one of its labels.
type SingleChoice struct{ labels []Label }

// Checklist answers each criterion of an item with one of a shared set of
// answers; its rule derives a verdict from those answers.
type Checklist struct {
	criteria []Criterion
	answers  []Label
	rule     AllPass
}

func (SingleChoice) isScheme() {}
func (Checklist) isScheme()    {}

// Criterion is one named question of a checklist.
type Criterion struct {
	Name  string
	Title string
}

// ChecklistSpec is the unvalidated input from which a Checklist is built.
type ChecklistSpec struct {
	Criteria []Criterion
	Answers  []Label
	Rule     AllPass
}

// Labels returns the labels in configured order.
func (s SingleChoice) Labels() []Label { return append([]Label(nil), s.labels...) }

// LabelByKey finds the label bound to key.
func (s SingleChoice) LabelByKey(key rune) (Label, bool) { return labelByKey(s.labels, key) }

func (s SingleChoice) hasLabel(name string) bool { return hasLabel(s.labels, name) }

// Criteria returns the criteria in configured order.
func (c Checklist) Criteria() []Criterion { return append([]Criterion(nil), c.criteria...) }

// Answers returns the answers every criterion chooses from.
func (c Checklist) Answers() []Label { return append([]Label(nil), c.answers...) }

// AnswerByKey finds the answer bound to key.
func (c Checklist) AnswerByKey(key rune) (Label, bool) { return labelByKey(c.answers, key) }

// Verdict applies the checklist's rule to answers, keyed by criterion name.
func (c Checklist) Verdict(answers map[string]string) Verdict {
	return c.rule.verdict(c.criteria, answers)
}

// Complete reports whether every criterion has an answer.
func (c Checklist) Complete(answers map[string]string) bool {
	return c.firstUnanswered(answers, 0) < 0
}

// firstUnanswered returns the index of the first unanswered criterion at or
// after from, wrapping around, or -1 when all are answered.
func (c Checklist) firstUnanswered(answers map[string]string, from int) int {
	n := len(c.criteria)
	for i := range n {
		idx := (from + i) % n
		if _, ok := answers[c.criteria[idx].Name]; !ok {
			return idx
		}
	}
	return -1
}

func (c Checklist) hasCriterion(name string) bool {
	for _, cr := range c.criteria {
		if cr.Name == name {
			return true
		}
	}
	return false
}

// VerdictRule names how a checklist's answers combine into a verdict.
type VerdictRule int

// Verdict rules.
const (
	// RuleAllPass passes when every criterion passes and fails when any fails.
	RuleAllPass VerdictRule = iota
)

func (r VerdictRule) String() string {
	switch r {
	case RuleAllPass:
		return "all_pass"
	}
	return fmt.Sprintf("VerdictRule(%d)", int(r))
}

// ParseVerdictRule is the inverse of VerdictRule.String.
func ParseVerdictRule(s string) (VerdictRule, error) {
	if s == RuleAllPass.String() {
		return RuleAllPass, nil
	}
	return 0, fmt.Errorf("unknown verdict rule %q", s)
}

// AllPass is the all_pass rule: the verdict is pass when every criterion has
// the Pass answer, fail when any criterion has the Fail answer, and undecided
// otherwise (an unanswered criterion or any other answer).
type AllPass struct {
	Pass string
	Fail string
}

func (r AllPass) verdict(criteria []Criterion, answers map[string]string) Verdict {
	passed := 0
	for _, c := range criteria {
		switch answers[c.Name] {
		case r.Fail:
			return VerdictFail
		case r.Pass:
			passed++
		}
	}
	if passed == len(criteria) {
		return VerdictPass
	}
	return VerdictUndecided
}

// Verdict is the outcome a checklist's rule derives from its answers.
type Verdict int

// Verdicts.
const (
	VerdictUndecided Verdict = iota
	VerdictPass
	VerdictFail
)

func (v Verdict) String() string {
	switch v {
	case VerdictUndecided:
		return "undecided"
	case VerdictPass:
		return "pass"
	case VerdictFail:
		return "fail"
	}
	return fmt.Sprintf("Verdict(%d)", int(v))
}

func newScheme(labels []Label, checklist *ChecklistSpec) (Scheme, error) {
	switch {
	case checklist != nil && len(labels) > 0:
		return nil, invalidTask("set labels or checklist, not both")
	case checklist != nil:
		return newChecklist(*checklist)
	}
	if err := validateLabels(labels, "label"); err != nil {
		return nil, err
	}
	return SingleChoice{labels: append([]Label(nil), labels...)}, nil
}

func newChecklist(spec ChecklistSpec) (Checklist, error) {
	if err := validateCriteria(spec.Criteria); err != nil {
		return Checklist{}, err
	}
	if err := validateLabels(spec.Answers, "answer"); err != nil {
		return Checklist{}, err
	}
	if err := validateAllPass(spec.Rule, spec.Answers); err != nil {
		return Checklist{}, err
	}
	return Checklist{
		criteria: append([]Criterion(nil), spec.Criteria...),
		answers:  append([]Label(nil), spec.Answers...),
		rule:     spec.Rule,
	}, nil
}

func validateCriteria(criteria []Criterion) error {
	if len(criteria) == 0 {
		return invalidTask("no criteria")
	}
	seen := map[string]bool{}
	for _, c := range criteria {
		if c.Name == "" {
			return invalidTask("criterion name is empty")
		}
		if seen[c.Name] {
			return invalidTask("duplicate criterion %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

func validateAllPass(r AllPass, answers []Label) error {
	if !hasLabel(answers, r.Pass) {
		return invalidTask("verdict rule %s: pass answer %q is not an answer", RuleAllPass, r.Pass)
	}
	if !hasLabel(answers, r.Fail) {
		return invalidTask("verdict rule %s: fail answer %q is not an answer", RuleAllPass, r.Fail)
	}
	if r.Pass == r.Fail {
		return invalidTask("verdict rule %s: pass and fail answers must differ", RuleAllPass)
	}
	return nil
}

// keyedLabels returns the labels that own single-character keys under s, and
// the word that names them in messages.
func keyedLabels(s Scheme) ([]Label, string) {
	switch s := s.(type) {
	case SingleChoice:
		return s.labels, "label"
	case Checklist:
		return s.answers, "answer"
	}
	return nil, ""
}

func labelByKey(labels []Label, key rune) (Label, bool) {
	for _, l := range labels {
		if l.Key == key {
			return l, true
		}
	}
	return Label{}, false
}

func hasLabel(labels []Label, name string) bool {
	for _, l := range labels {
		if l.Name == name {
			return true
		}
	}
	return false
}
