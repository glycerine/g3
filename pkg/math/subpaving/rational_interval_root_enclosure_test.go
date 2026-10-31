package subpaving

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/clibrand"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Odd-degree XnEqY uses the source interval nth_root path.
func TestSubpavingCubeRootIntervalEnclosures(t *testing.T) {
	budget := arena.NewBudget(1 << 27)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewRationalInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewRationalInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	precision := numeral.NewRational()
	t.Cleanup(func() {
		for _, err := range []error{out.Close(), a.Close(), m.Del(&precision), m.CloseChecked()} {
			if err != nil {
				t.Error(err)
			}
		}
		if budget.Used() != 0 {
			t.Error("numeric storage was not released", budget.Used())
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.SetFractionSigned(&precision, 1, 100))
	roots := interval.RationalRoot{Numbers: m, Limit: &control.Reslimit{Limit: 1000000}}
	var rng clibrand.GLibc31
	for i := 0; i < 500; i++ {
		lo := int64(rng.Next()%21) - 10
		hi := lo + int64(rng.Next()%11)
		lowerInf, upperInf := rng.Next()%4 == 0, rng.Next()%4 == 0
		lowerOpen, upperOpen := rng.Next()%2 == 0, rng.Next()%2 == 0
		if lo == hi && !lowerInf && !upperInf {
			lowerOpen, upperOpen = false, false
		}
		must(m.SetInt64(&a.lower, lo))
		must(m.SetInt64(&a.upper, hi))
		a.lowerInf, a.upperInf = lowerInf, upperInf
		a.lowerOpen, a.upperOpen = lowerInf || lowerOpen, upperInf || upperOpen
		must(a.CheckInvariant())
		must(out.XnEqY(a, 3, &precision, roots))
		must(out.CheckInvariant())
		if out.lowerInf != lowerInf || out.upperInf != upperInf {
			t.Fatalf("case %d: infinity flags changed", i)
		}
		for _, b := range []struct {
			input                      int64
			inputOpen, inf, outputOpen bool
			value                      *numeral.Rational
			lower                      bool
		}{
			{lo, a.lowerOpen, lowerInf, out.lowerOpen, &out.lower, true},
			{hi, a.upperOpen, upperInf, out.upperOpen, &out.upper, false},
		} {
			if b.inf {
				if !b.outputOpen {
					t.Fatalf("case %d: closed infinity", i)
				}
				continue
			}
			s, err := m.String(b.value)
			must(err)
			x, ok := new(big.Rat).SetString(s)
			if !ok {
				t.Fatal("invalid rational", s)
			}
			cube := new(big.Rat).Mul(x, x)
			cube.Mul(cube, x)
			cmp := cube.Cmp(new(big.Rat).SetInt64(b.input))
			if b.lower && cmp > 0 || !b.lower && cmp < 0 || b.outputOpen != (b.inputOpen && cmp == 0) {
				t.Fatalf("case %d: root bound %s for %d, comparison %d, open %t", i, x, b.input, cmp, b.outputOpen)
			}
		}
	}
}
