package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type boundedSignFixture struct {
	t        *testing.T
	budget   *arena.Budget
	service  *numeral.RationalService
	bounds   *numeral.BinaryRationalManager
	store    *realValueStore
	interval *BinaryInterval
	lower    numeral.BinaryRational
	upper    numeral.BinaryRational
	values   []*realValue
}

func newBoundedSignFixture(t *testing.T, lower, upper int32) *boundedSignFixture {
	t.Helper()
	f := &boundedSignFixture{t: t, budget: arena.NewBudget(1 << 27), lower: numeral.NewBinaryRational(), upper: numeral.NewBinaryRational()}
	var err error
	f.service, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: f.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.bounds, err = numeral.NewBinaryRationalManager(f.service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: f.service})
	if err != nil {
		t.Fatal(err)
	}
	f.store = &realValueStore{numbers: f.service.Manager}
	f.interval, err = NewBinaryInterval(f.bounds)
	if err != nil {
		t.Fatal(err)
	}
	for _, setupErr := range []error{f.bounds.SetInt32(&f.lower, lower), f.bounds.SetInt32(&f.upper, upper), f.interval.SetFinite(&f.lower, &f.upper, false, false)} {
		if setupErr != nil {
			t.Fatal(setupErr)
		}
	}
	t.Cleanup(f.close)
	return f
}

func (f *boundedSignFixture) rational(n int64) *realValue {
	f.t.Helper()
	v, err := f.store.newRational(n, 1)
	if err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, v)
	return v
}

func (f *boundedSignFixture) close() {
	f.t.Helper()
	if err := f.store.cleanupPendingPolynomialSequence(); err != nil {
		f.t.Error(err)
	}
	for _, value := range f.values {
		if !value.deleted {
			if err := f.store.release(value); err != nil {
				f.t.Error(err)
			}
		}
	}
	for _, err := range []error{f.interval.Close(), f.bounds.Del(&f.lower), f.bounds.Del(&f.upper), f.bounds.CloseChecked(), f.service.Close()} {
		if err != nil {
			f.t.Error(err)
		}
	}
	if used := f.budget.Used(); used != 0 {
		f.t.Error("bounded sign fixture retained bytes", used)
	}
}

func TestBoundedRationalSignDeterminationThreeRootPath(t *testing.T) {
	for _, usePrem := range []bool{true, false} {
		t.Run(map[bool]string{true: "pseudo", false: "remainder"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, -2, 2)
			one, minusOne := f.rational(1), f.rational(-1)
			p := []*realValue{nil, minusOne, nil, one} // x^3-x
			q := []*realValue{nil, one}                // x
			state := newSignDetermination(3)
			if err := state.attachInitialPolynomial(f.store, one); err != nil {
				t.Fatal(err)
			}
			kept, allOne, err := state.advanceBoundedRational(p, q, f.interval, usePrem)
			if err != nil || !kept || !allOne {
				t.Fatal("three-root transition", kept, allOne, err)
			}
			if state.matrix.rows != 3 || state.matrix.cols != 3 || len(state.prs) != 3 || len(state.qs) != 1 || len(state.conditions) != 3 || state.polynomials != 1 {
				t.Fatal("three-root state dimensions")
			}
			if got, ok := state.matrix.solveInt(state.tarski); !ok || len(got) != 3 || got[0] != 1 || got[1] != 1 || got[2] != 1 {
				t.Fatal("three-root cardinalities", got, ok)
			}
			if err := state.closePolynomials(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBoundedRationalSignDeterminationTwoStepFourRootPath(t *testing.T) {
	for _, usePrem := range []bool{true, false} {
		t.Run(map[bool]string{true: "pseudo", false: "remainder"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, -3, 3)
			one, minusTwo, minusFive, four := f.rational(1), f.rational(-2), f.rational(-5), f.rational(4)
			p := []*realValue{four, nil, minusFive, nil, one} // x^4-5x^2+4
			first := []*realValue{nil, one}                   // x
			second := []*realValue{minusTwo, nil, one}        // x^2-2
			state := newSignDetermination(4)
			if err := state.attachInitialPolynomial(f.store, one); err != nil {
				t.Fatal(err)
			}
			kept, allOne, err := state.advanceBoundedRational(p, first, f.interval, usePrem)
			if err != nil || !kept || allOne || state.matrix.rows != 2 || len(state.prs) != 2 || len(state.qs) != 1 {
				t.Fatal("first four-root transition", kept, allOne, err, state.matrix.rows, len(state.prs))
			}
			kept, allOne, err = state.advanceBoundedRational(p, second, f.interval, usePrem)
			if err != nil || !kept || !allOne || state.matrix.rows != 4 || len(state.prs) != 4 || len(state.qs) != 2 || state.polynomials != 2 {
				t.Fatal("second four-root transition", kept, allOne, err, state.matrix.rows, len(state.prs))
			}
			if got, ok := state.matrix.solveInt(state.tarski); !ok || len(got) != 4 || got[0] != 1 || got[1] != 1 || got[2] != 1 || got[3] != 1 {
				t.Fatal("four-root cardinalities", got, ok)
			}
			if err := state.closePolynomials(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBoundedRationalSignDeterminationRejectsUselessPolynomial(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	one, minusOne := f.rational(1), f.rational(-1)
	p := []*realValue{nil, minusOne, nil, one}
	state := newSignDetermination(3)
	if err := state.attachInitialPolynomial(f.store, one); err != nil {
		t.Fatal(err)
	}
	kept, allOne, err := state.advanceBoundedRational(p, []*realValue{one}, f.interval, true)
	if err != nil || kept || allOne || state.matrix.rows != 1 || len(state.prs) != 1 || len(state.qs) != 0 || state.polynomials != 0 {
		t.Fatal("useless positive polynomial mutated state", kept, allOne, err)
	}
	if err := state.closePolynomials(); err != nil {
		t.Fatal(err)
	}
}
