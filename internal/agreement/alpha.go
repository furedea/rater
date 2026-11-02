package agreement

// AlphaResult is Krippendorff's alpha with the data it was computed from:
// Units is the number of pairable units (rated by at least two raters) and
// Values the number of values in them.
type AlphaResult struct {
	Units  int
	Values int
	Alpha  Coefficient
}

// Alpha computes Krippendorff's alpha for nominal data with missing values,
// alpha = 1 - D_o / D_e (Krippendorff, "Computing Krippendorff's
// Alpha-Reliability", 2011).
//
// With n_c the number of pairable values in category c, n their total, and
// m_u the number of values in unit u, the coincidence sums reduce to
//
//	D_o ∝ Σ_u (m_u² - Σ_c n_uc²) / (m_u - 1)
//	D_e ∝ (n² - Σ_c n_c²) / (n - 1)
//
// so no coincidence matrix needs to be built.
func Alpha(d Data) AlphaResult {
	totals := map[string]int{}
	var r AlphaResult
	observed := 0.0
	for _, u := range d.units {
		values := presentValues(u)
		m := len(values)
		if m < 2 {
			continue
		}
		counts := countCategories(values)
		for c, n := range counts {
			totals[c] += n
		}
		observed += float64(m*m-sumOfSquares(counts)) / float64(m-1)
		r.Units++
		r.Values += m
	}
	switch {
	case r.Units == 0:
		r.Alpha = undefined(ReasonNoOverlap)
		return r
	case len(totals) == 1:
		r.Alpha = undefined(ReasonSingleCategory)
		return r
	}
	n := r.Values
	expected := float64(n*n-sumOfSquares(totals)) / float64(n-1)
	r.Alpha = defined(1 - observed/expected)
	return r
}

func countCategories(values []string) map[string]int {
	counts := map[string]int{}
	for _, c := range values {
		counts[c]++
	}
	return counts
}

func sumOfSquares(counts map[string]int) int {
	total := 0
	for _, n := range counts {
		total += n * n
	}
	return total
}
