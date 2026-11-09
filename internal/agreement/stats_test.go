package agreement_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/agreement"
)

// published values are printed to three decimals, so they match to half a unit there.
const printedPrecision = 0.0005

// exact is the tolerance for values computed from small integer counts.
const exact = 1e-12

// unitsFrom builds units from one string per rater; "." is a missing value.
func unitsFrom(t *testing.T, raters ...string) agreement.Data {
	t.Helper()
	columns := make([][]string, len(raters))
	for i, r := range raters {
		columns[i] = strings.Fields(r)
		require.Len(t, columns[i], len(columns[0]), "rater %d", i)
	}
	units := make([][]agreement.Value, len(columns[0]))
	for u := range units {
		for _, col := range columns {
			units[u] = append(units[u], value(col[u]))
		}
	}
	d, err := agreement.NewData(len(raters), units)
	require.NoError(t, err)
	return d
}

func value(s string) agreement.Value {
	if s == "." {
		return agreement.Missing
	}
	return agreement.Category(s)
}

// pairFromCounts builds two-rater data from a confusion table: counts[i][j]
// units where rater A chose categories[i] and rater B chose categories[j].
func pairFromCounts(t *testing.T, categories []string, counts [][]int) agreement.Data {
	t.Helper()
	var units [][]agreement.Value
	for i, row := range counts {
		for j, n := range row {
			for range n {
				units = append(units, []agreement.Value{agreement.Category(categories[i]), agreement.Category(categories[j])})
			}
		}
	}
	d, err := agreement.NewData(2, units)
	require.NoError(t, err)
	return d
}

func requireValue(t *testing.T, c agreement.Coefficient) float64 {
	t.Helper()
	v, ok := c.Value()
	require.True(t, ok, "coefficient is undefined: %s", c.Reason())
	return v
}

// Wikipedia, "Cohen's kappa", section "Simple example": 50 grant proposals
// read by A and B; Yes/Yes 20, Yes/No 5, No/Yes 10, No/No 15 gives
// p_o = 0.70, p_e = 0.50 and kappa = 0.40.
func TestKappaMatchesWikipediaGrantProposalExample(t *testing.T) {
	t.Parallel()

	d := pairFromCounts(t, []string{"yes", "no"}, [][]int{{20, 5}, {10, 15}})

	c, err := agreement.NewConfusion(d)
	require.NoError(t, err)
	k := c.Kappa()

	assert.Equal(t, 50, k.Units)
	assert.InDelta(t, 0.70, k.Observed, exact)
	assert.InDelta(t, 0.50, k.Expected, exact)
	assert.InDelta(t, 0.40, requireValue(t, k.Kappa), exact)
}

// R package irr, ?kappa2 example: kappa2(diagnoses[,2:3]) reports 30 subjects
// and unweighted kappa 0.631. diagnoses is Fleiss (1971); the columns below
// are rater2 and rater3 decoded from irr's data/diagnoses.rda, with categories
// 1 Depression, 2 Personality Disorder, 3 Schizophrenia, 4 Neurosis, 5 Other.
func TestKappaMatchesIrrKappa2DiagnosesExample(t *testing.T) {
	t.Parallel()

	d := unitsFrom(t,
		"4 2 3 5 2 1 3 1 1 5 4 2 2 4 2 3 1 1 2 3 5 4 2 1 4 2 1 2 3 5",
		"4 2 3 5 2 3 3 3 4 5 4 4 2 4 4 3 1 1 4 3 5 4 4 4 4 2 1 4 3 5",
	)

	c, err := agreement.NewConfusion(d)
	require.NoError(t, err)
	k := c.Kappa()

	assert.Equal(t, 30, k.Units)
	assert.InDelta(t, 0.70, k.Observed, exact)
	assert.InDelta(t, 0.631, requireValue(t, k.Kappa), printedPrecision)
}

// R package irr, ?kripp.alpha example: the 4 raters x 12 subjects matrix nmm
// (Krippendorff's own example with missing values) has nominal alpha 0.743.
// Rows below are the raters; "." is NA.
func TestAlphaMatchesIrrKrippAlphaExample(t *testing.T) {
	t.Parallel()

	d := unitsFrom(t,
		"1 2 3 3 2 1 4 1 2 . . .",
		"1 2 3 3 2 2 4 1 2 5 . 3",
		". 3 3 3 2 3 4 2 2 5 1 .",
		"1 2 3 3 2 4 4 1 2 5 1 .",
	)

	a := agreement.Alpha(d)

	assert.Equal(t, 11, a.Units, "subject 12 has one value and is not pairable")
	assert.Equal(t, 40, a.Values)
	assert.InDelta(t, 0.743, requireValue(t, a.Alpha), printedPrecision)
}

func TestConfusionCountsUnitsRatedByBoth(t *testing.T) {
	t.Parallel()

	d := unitsFrom(t,
		"a a b b . c",
		"a b b b c .",
	)

	c, err := agreement.NewConfusion(d)
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b"}, c.Categories(), "categories are sorted and only from compared units")
	assert.Equal(t, 1, c.Count("a", "a"))
	assert.Equal(t, 1, c.Count("a", "b"))
	assert.Equal(t, 0, c.Count("b", "a"))
	assert.Equal(t, 2, c.Count("b", "b"))
	assert.Equal(t, 0, c.Count("c", "c"), "unknown categories count zero")
	assert.Equal(t, 4, c.Kappa().Units)
}

func TestNewConfusionNeedsExactlyTwoRaters(t *testing.T) {
	t.Parallel()

	_, err := agreement.NewConfusion(unitsFrom(t, "a", "a", "a"))

	assert.ErrorIs(t, err, agreement.ErrNotPair)
}

func TestNewDataRejectsRaggedUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raters int
		units  [][]agreement.Value
	}{
		{"no raters", 0, nil},
		{"short unit", 2, [][]agreement.Value{{agreement.Category("a")}}},
		{"long unit", 1, [][]agreement.Value{{agreement.Category("a"), agreement.Missing}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := agreement.NewData(tt.raters, tt.units)
			assert.ErrorIs(t, err, agreement.ErrShape)
		})
	}
}

func TestKappaDegenerateInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		a, b       string
		wantReason agreement.Reason
		wantUnits  int
	}{
		{"no unit rated by both", "a . a", ". b .", agreement.ReasonNoOverlap, 0},
		{"no units at all", "", "", agreement.ReasonNoOverlap, 0},
		{"one shared category means expected agreement is 1", "a a . a", "a a b a", agreement.ReasonSingleCategory, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := agreement.NewConfusion(unitsFrom(t, tt.a, tt.b))
			require.NoError(t, err)
			k := c.Kappa()

			_, ok := k.Kappa.Value()
			assert.False(t, ok)
			assert.Equal(t, tt.wantReason, k.Kappa.Reason())
			assert.Equal(t, tt.wantUnits, k.Units)
		})
	}
}

func TestKappaOfOppositeRatersIsNegative(t *testing.T) {
	t.Parallel()

	c, err := agreement.NewConfusion(unitsFrom(t, "a b a b", "b a b a"))
	require.NoError(t, err)

	assert.InDelta(t, -1.0, requireValue(t, c.Kappa().Kappa), exact)
}

func TestAlphaDegenerateInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raters     []string
		wantReason agreement.Reason
		wantUnits  int
	}{
		{"no pairable unit", []string{"a . b", ". c ."}, agreement.ReasonNoOverlap, 0},
		{"single rater", []string{"a b c"}, agreement.ReasonNoOverlap, 0},
		{"one category means no expected disagreement", []string{"a a .", "a a b", "a . ."}, agreement.ReasonSingleCategory, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := agreement.Alpha(unitsFrom(t, tt.raters...))

			_, ok := a.Alpha.Value()
			assert.False(t, ok)
			assert.Equal(t, tt.wantReason, a.Alpha.Reason())
			assert.Equal(t, tt.wantUnits, a.Units)
		})
	}
}

// Missing values only remove a rater from a unit: the same pairable values
// give the same alpha whatever else is missing around them.
func TestAlphaIgnoresUnpairableValues(t *testing.T) {
	t.Parallel()

	full := agreement.Alpha(unitsFrom(t, "a b a b", "a b b b"))
	padded := agreement.Alpha(unitsFrom(t, "a b a b c .", "a b b b . d", ". . . . . ."))

	assert.InDelta(t, requireValue(t, full.Alpha), requireValue(t, padded.Alpha), exact)
	assert.Equal(t, full.Units, padded.Units)
}

func TestPerfectAgreementIsOne(t *testing.T) {
	t.Parallel()

	d := unitsFrom(t, "a b c a", "a b c a")
	c, err := agreement.NewConfusion(d)
	require.NoError(t, err)

	assert.InDelta(t, 1.0, requireValue(t, c.Kappa().Kappa), exact)
	assert.InDelta(t, 1.0, requireValue(t, agreement.Alpha(d).Alpha), exact)
}
