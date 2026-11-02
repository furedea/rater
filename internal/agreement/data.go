// Package agreement computes inter-annotator agreement on nominal data:
// Cohen's kappa and a confusion matrix for two raters, and Krippendorff's
// alpha for any number of raters with missing values. Report applies them
// to a task's labels, checklist criteria, and derived verdicts. It does no I/O.
package agreement

import (
	"errors"
	"fmt"
)

// Value is one rater's nominal value for a unit, or Missing.
type Value struct {
	category string
	present  bool
}

// Missing marks a unit a rater gave no value for.
var Missing = Value{}

// Category returns a present value naming category c.
func Category(c string) Value { return Value{category: c, present: true} }

// Data is a reliability data matrix: every unit holds one value per rater.
type Data struct {
	raters int
	units  [][]Value
}

// ErrShape means a unit does not hold exactly one value per rater.
var ErrShape = errors.New("units must hold one value per rater")

// NewData checks that each unit has one value per rater; there is at least one rater.
func NewData(raters int, units [][]Value) (Data, error) {
	if raters < 1 {
		return Data{}, fmt.Errorf("%w: %d raters", ErrShape, raters)
	}
	copied := make([][]Value, len(units))
	for i, u := range units {
		if len(u) != raters {
			return Data{}, fmt.Errorf("%w: unit %d has %d values for %d raters", ErrShape, i+1, len(u), raters)
		}
		copied[i] = append([]Value(nil), u...)
	}
	return Data{raters: raters, units: copied}, nil
}

// presentValues returns the categories given in unit u, in rater order.
func presentValues(u []Value) []string {
	var cs []string
	for _, v := range u {
		if v.present {
			cs = append(cs, v.category)
		}
	}
	return cs
}

// Reason says why a coefficient has no value.
type Reason int

// Reasons. ReasonNone means the coefficient is defined.
const (
	ReasonNone Reason = iota
	// ReasonNoOverlap: no unit was rated by enough raters to compare.
	ReasonNoOverlap
	// ReasonSingleCategory: every compared value is the same category, so
	// chance agreement is perfect and the coefficient is 0/0.
	ReasonSingleCategory
)

func (r Reason) String() string {
	switch r {
	case ReasonNone:
		return "defined"
	case ReasonNoOverlap:
		return "no overlapping items"
	case ReasonSingleCategory:
		return "single category"
	}
	return fmt.Sprintf("Reason(%d)", int(r))
}

// Coefficient is an agreement coefficient, or the reason it is undefined.
type Coefficient struct {
	value  float64
	reason Reason
}

func defined(v float64) Coefficient       { return Coefficient{value: v} }
func undefined(reason Reason) Coefficient { return Coefficient{reason: reason} }

// Value returns the coefficient and whether it is defined.
func (c Coefficient) Value() (float64, bool) { return c.value, c.reason == ReasonNone }

// Reason returns why the coefficient is undefined, or ReasonNone.
func (c Coefficient) Reason() Reason { return c.reason }
