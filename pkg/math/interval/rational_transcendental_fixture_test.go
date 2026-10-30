package interval

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type rationalTranscendentalFixture struct {
	t      *testing.T
	budget *arena.Budget
	number *numeral.RationalManager
	owner  RationalTranscendentals
	values []*numeral.Rational
	ranges []*RationalInterval
}

func newRationalTranscendentalFixture(t *testing.T) *rationalTranscendentalFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 30)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f := &rationalTranscendentalFixture{t: t, budget: budget, number: m}
	f.owner.Numbers = m
	return f
}

func (f *rationalTranscendentalFixture) rational(text string) *numeral.Rational {
	f.t.Helper()
	v := numeral.NewRational()
	if err := f.number.SetString(&v, text); err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, &v)
	return &v
}

func (f *rationalTranscendentalFixture) interval() *RationalInterval {
	f.t.Helper()
	v, err := NewRationalInterval(f.number)
	if err != nil {
		f.t.Fatal(err)
	}
	f.ranges = append(f.ranges, v)
	return v
}

func (f *rationalTranscendentalFixture) close() {
	f.t.Helper()
	if err := f.owner.Close(); err != nil {
		f.t.Fatal(err)
	}
	for i := len(f.ranges) - 1; i >= 0; i-- {
		if err := f.ranges[i].Close(); err != nil {
			f.t.Fatal(err)
		}
	}
	for i := len(f.values) - 1; i >= 0; i-- {
		if err := f.number.Del(f.values[i]); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := f.number.CloseChecked(); err != nil {
		f.t.Fatal(err)
	}
	if used := f.budget.Used(); used != 0 {
		f.t.Fatalf("transcendental fixture retained %d bytes", used)
	}
}

func (f *rationalTranscendentalFixture) encloses(v *RationalInterval, text string) bool {
	f.t.Helper()
	x := f.rational(text)
	ok, err := v.Contains(x)
	if err != nil {
		f.t.Fatal(err)
	}
	return ok
}
