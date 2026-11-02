package agreement

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// Confusion is the contingency table of two raters over the units both rated:
// rows are the first rater's categories, columns the second's.
type Confusion struct {
	categories []string
	counts     map[[2]string]int
	units      int
}

// ErrNotPair means a two-rater statistic was asked of other data.
var ErrNotPair = errors.New("needs exactly two raters")

// NewConfusion tabulates d, which must have exactly two raters. Units missing
// either value are left out.
func NewConfusion(d Data) (Confusion, error) {
	if d.raters != pair {
		return Confusion{}, fmt.Errorf("%w: got %d", ErrNotPair, d.raters)
	}
	c := Confusion{counts: map[[2]string]int{}}
	seen := map[string]bool{}
	for _, u := range d.units {
		a, b := u[0], u[1]
		if !a.present || !b.present {
			continue
		}
		c.counts[[2]string{a.category, b.category}]++
		c.units++
		seen[a.category], seen[b.category] = true, true
	}
	c.categories = slices.Sorted(maps.Keys(seen))
	return c, nil
}

// Categories returns the categories either rater used, sorted.
func (c Confusion) Categories() []string { return slices.Clone(c.categories) }

// Count returns how many units the first rater put in row and the second in col.
func (c Confusion) Count(row, col string) int { return c.counts[[2]string{row, col}] }

// KappaResult is Cohen's kappa with the agreement it is made from. Observed
// and Expected are proportions of Units and are zero when Units is zero.
type KappaResult struct {
	Units    int
	Observed float64
	Expected float64
	Kappa    Coefficient
}

// Kappa computes Cohen's (1960) unweighted kappa,
// (p_o - p_e) / (1 - p_e), from the table.
func (c Confusion) Kappa() KappaResult {
	if c.units == 0 {
		return KappaResult{Kappa: undefined(ReasonNoOverlap)}
	}
	agreed, chance := 0, 0
	for _, k := range c.categories {
		agreed += c.Count(k, k)
		chance += c.rowTotal(k) * c.colTotal(k)
	}
	n := float64(c.units)
	r := KappaResult{
		Units:    c.units,
		Observed: float64(agreed) / n,
		Expected: float64(chance) / (n * n),
	}
	// Compare the integer counts: p_e is exactly 1 only when both raters
	// used one and the same category for every unit.
	if chance == c.units*c.units {
		r.Kappa = undefined(ReasonSingleCategory)
		return r
	}
	r.Kappa = defined((r.Observed - r.Expected) / (1 - r.Expected))
	return r
}

func (c Confusion) rowTotal(row string) int {
	total := 0
	for _, col := range c.categories {
		total += c.Count(row, col)
	}
	return total
}

func (c Confusion) colTotal(col string) int {
	total := 0
	for _, row := range c.categories {
		total += c.Count(row, col)
	}
	return total
}
