package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type realFunctionFixture struct {
	t         *testing.T
	budget    *arena.Budget
	service   *numeral.RationalService
	intervals *numeral.BinaryRationalManager
	store     *realValueStore
	owner     *algebraicRootOwner
	root      *algebraicRoot
	alpha     *realValue
	one       *realValue
	minusTwo  *realValue
	enclosure *BinaryInterval
	lo, hi    numeral.BinaryRational
}

func newRealFunctionFixture(t *testing.T) *realFunctionFixture {
	t.Helper()
	f := &realFunctionFixture{t: t, budget: arena.NewBudget(1 << 25)}
	var err error
	f.service, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: f.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.intervals, err = numeral.NewBinaryRationalManager(f.service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: f.service})
	if err != nil {
		t.Fatal(err)
	}
	f.store = &realValueStore{numbers: f.service.Manager}
	f.one, err = f.store.newRational(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.minusTwo, err = f.store.newRational(-2, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.lo, f.hi = numeral.NewBinaryRational(), numeral.NewBinaryRational()
	if err = f.intervals.SetInt32(&f.lo, 1); err != nil {
		t.Fatal(err)
	}
	if err = f.intervals.SetInt32(&f.hi, 2); err != nil {
		t.Fatal(err)
	}
	f.enclosure, err = NewBinaryInterval(f.intervals)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.enclosure.SetFinite(&f.lo, &f.hi, false, false); err != nil {
		t.Fatal(err)
	}
	f.owner = &algebraicRootOwner{values: f.store, intervals: f.intervals}
	f.root, err = f.owner.mkAlgebraic([]*realValue{f.minusTwo, nil, f.one}, f.enclosure, f.enclosure, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	f.alpha, err = f.store.newAlgebraicValue(f.root, f.one)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		mustRealFunction(t, f.store.release(f.alpha))
		mustRealFunction(t, f.store.release(f.one))
		mustRealFunction(t, f.store.release(f.minusTwo))
		mustRealFunction(t, f.enclosure.Close())
		mustRealFunction(t, f.intervals.Del(&f.lo))
		mustRealFunction(t, f.intervals.Del(&f.hi))
		mustRealFunction(t, f.intervals.CloseChecked())
		mustRealFunction(t, f.service.Close())
		if used := f.budget.Used(); used != 0 {
			t.Fatalf("real function fixture retained %d bytes", used)
		}
	})
	return f
}

func mustRealFunction(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *realFunctionFixture) own(v *realValue) *realValue {
	f.t.Helper()
	f.t.Cleanup(func() { mustRealFunction(f.t, f.store.release(v)) })
	return v
}

func (f *realFunctionFixture) rational(n int64) *realValue {
	f.t.Helper()
	v, err := f.store.newRational(n, 1)
	mustRealFunction(f.t, err)
	return f.own(v)
}

func (f *realFunctionFixture) coefficientText(v *realValue) string {
	f.t.Helper()
	if v == nil {
		return "0"
	}
	if !v.isRational {
		f.t.Fatal("coefficient is not rational")
	}
	text, err := f.store.numbers.String(&v.rational)
	mustRealFunction(f.t, err)
	return text
}
