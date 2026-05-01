package jinja

import (
	"testing"
)

// measure compares the allocations of two templates rendered with the
// same data. Returns (with - without) — the cost attributable to the
// extra slice expression.
func measure(t *testing.T, withExpr, withoutExpr string, items int) (delta float64) {
	t.Helper()

	mk := func(src string) *Template {
		tmpl, err := Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		return tmpl
	}

	xs := make([]any, items)
	for i := range xs {
		xs[i] = i
	}
	data := map[string]any{"items": xs}

	with := mk(withExpr)
	without := mk(withoutExpr)

	for range 5 {
		_, _ = with.Render(data)
		_, _ = without.Render(data)
	}

	a := testing.AllocsPerRun(2000, func() {
		_, _ = with.Render(data)
	})
	b := testing.AllocsPerRun(2000, func() {
		_, _ = without.Render(data)
	})
	t.Logf("with=%.2f without=%.2f delta=%.2f", a, b, a-b)
	return a - b
}

// TestEvalSliceAllocs_Step1 verifies the step==1 fast path allocates the
// minimum: one []Value backing array + one *List wrapper = 2 allocs.
func TestEvalSliceAllocs_Step1(t *testing.T) {
	delta := measure(t,
		`{% set tail = items[1:] %}{{ tail|length }}`,
		`{{ items|length }}`,
		64,
	)
	if delta > 3.0 {
		t.Errorf("step==1 slice should add ≤3 allocs (slice + *List + ?), got delta=%.2f", delta)
	}
}

// TestEvalSliceAllocs_StepN verifies the general step path is also at
// minimum cost when the pre-size is exact.
func TestEvalSliceAllocs_StepN(t *testing.T) {
	delta := measure(t,
		`{% set evens = items[0:64:2] %}{{ evens|length }}`,
		`{{ items|length }}`,
		64,
	)
	if delta > 3.0 {
		t.Errorf("step==2 slice with pre-size should add ≤3 allocs, got delta=%.2f", delta)
	}
}

// TestEvalSliceAllocs_NegStart verifies the start<0 case is still bound
// by pre-sizing (no append growth).
func TestEvalSliceAllocs_NegStart(t *testing.T) {
	delta := measure(t,
		`{% set tail = items[-10:] %}{{ tail|length }}`,
		`{{ items|length }}`,
		64,
	)
	if delta > 3.0 {
		t.Errorf("negative start should add ≤3 allocs, got delta=%.2f", delta)
	}
}
