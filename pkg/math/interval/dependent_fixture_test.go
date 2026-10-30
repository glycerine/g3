package interval

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type dependentIntervalFixture struct {
	t       *testing.T
	budget  *arena.Budget
	numbers *numeral.RationalManager
	manager *DependentIntervalManager
	values  []*DependentRationalInterval
}

func newDependentIntervalFixture(t *testing.T) *dependentIntervalFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 27)
	numbers, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewDependentIntervalManager(numbers)
	if err != nil {
		t.Fatal(err)
	}
	f := &dependentIntervalFixture{t: t, budget: budget, numbers: numbers, manager: manager}
	t.Cleanup(func() {
		for i := len(f.values) - 1; i >= 0; i-- {
			if err := f.values[i].Close(); err != nil {
				t.Error(err)
			}
		}
		if err := numbers.CloseChecked(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("dependent intervals retained budget", budget.Used())
		}
	})
	return f
}

func (f *dependentIntervalFixture) interval(lower, upper string, lowerOpen, upperOpen bool, lowerDep, upperDep uint32) *DependentRationalInterval {
	f.t.Helper()
	v, err := f.manager.NewInterval()
	if err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, v)
	l, u := numeral.NewRational(), numeral.NewRational()
	if err = f.numbers.SetString(&l, lower); err == nil {
		err = f.numbers.SetString(&u, upper)
	}
	if err == nil {
		err = v.Value.SetFinite(&l, &u, lowerOpen, upperOpen)
	}
	if e := f.numbers.Del(&l); err == nil {
		err = e
	}
	if e := f.numbers.Del(&u); err == nil {
		err = e
	}
	if err != nil {
		f.t.Fatal(err)
	}
	v.LowerDep, v.UpperDep = f.manager.Leaf(lowerDep), f.manager.Leaf(upperDep)
	return v
}

func (f *dependentIntervalFixture) empty() *DependentRationalInterval {
	f.t.Helper()
	v, err := f.manager.NewInterval()
	if err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, v)
	return v
}
func (f *dependentIntervalFixture) rational(text string) numeral.Rational {
	f.t.Helper()
	v := numeral.NewRational()
	if err := f.numbers.SetString(&v, text); err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		if err := f.numbers.Del(&v); err != nil {
			f.t.Error(err)
		}
	})
	return v
}
