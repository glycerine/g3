package interval

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/clibrand"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Check the source display_lemmas implication shape over an independent grid:
// only reported input bounds may be used to justify each result bound.
func TestIntervalDependencyImplicationsOnRationalGrid(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	a, b, result := f.interval(), f.interval(), f.interval()
	set := func(dst *RationalInterval, src sourceRandomInterval) {
		t.Helper()
		if err := f.number.SetInt64(&dst.lower, src.lower); err != nil {
			t.Fatal(err)
		}
		if err := f.number.SetInt64(&dst.upper, src.upper); err != nil {
			t.Fatal(err)
		}
		dst.lowerInf, dst.upperInf, dst.lowerOpen, dst.upperOpen = src.lowerInf, src.upperInf, src.lowerOpen, src.upperOpen
		if err := dst.CheckInvariant(); err != nil {
			t.Fatal(err)
		}
	}
	toRat := func(value *numeral.Rational) *big.Rat {
		t.Helper()
		text, err := f.number.String(value)
		if err != nil {
			t.Fatal(err)
		}
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatal("invalid rational", text)
		}
		return r
	}
	allows := func(v sourceRandomInterval, value *big.Rat, lower, upper bool) bool {
		if lower && !v.lowerInf {
			cmp := value.Cmp(new(big.Rat).SetInt64(v.lower))
			if cmp < 0 || cmp == 0 && v.lowerOpen {
				return false
			}
		}
		if upper && !v.upperInf {
			cmp := value.Cmp(new(big.Rat).SetInt64(v.upper))
			if cmp > 0 || cmp == 0 && v.upperOpen {
				return false
			}
		}
		return true
	}
	var rng clibrand.GLibc31
	var checks int
	for _, op := range []string{"add", "sub", "mul", "neg", "pow2", "pow3", "inv", "div"} {
		for i := 0; i < 100; i++ {
			x, y := sourceIntervalRandom(&rng, 3), sourceIntervalRandom(&rng, 3)
			if op == "inv" {
				for x.hasZero() {
					x = sourceIntervalRandom(&rng, 3)
				}
			}
			if op == "div" {
				for y.hasZero() {
					y = sourceIntervalRandom(&rng, 3)
				}
			}
			set(a, x)
			set(b, y)
			var deps IntervalDeps
			var err error
			switch op {
			case "add":
				deps, err = a.AddDeps(b)
				if err == nil {
					err = result.Add(a, b)
				}
			case "sub":
				deps, err = a.SubDeps(b)
				if err == nil {
					err = result.Sub(a, b)
				}
			case "mul":
				deps, err = a.MulDeps(b)
				if err == nil {
					err = result.Mul(a, b)
				}
			case "neg":
				deps, err = a.NegDeps()
				if err == nil {
					err = result.Neg(a)
				}
			case "pow2", "pow3":
				degree := uint32(2)
				if op == "pow3" {
					degree = 3
				}
				deps, err = a.PowerDeps(degree)
				if err == nil {
					err = result.Power(a, degree)
				}
			case "inv":
				deps, err = a.InvDeps()
				if err == nil {
					err = result.Inv(a)
				}
			case "div":
				deps, err = a.DivDeps(b)
				if err == nil {
					err = result.Div(a, b)
				}
			}
			if err != nil {
				t.Fatal(op, i, err)
			}
			for _, bound := range []struct {
				deps             BoundDeps
				inf, open, lower bool
				value            *numeral.Rational
			}{
				{deps.Lower, result.lowerInf, result.lowerOpen, true, &result.lower},
				{deps.Upper, result.upperInf, result.upperOpen, false, &result.upper},
			} {
				if bound.inf {
					continue
				}
				endpoint := toRat(bound.value)
				for xi := int64(-10); xi <= 10; xi++ {
					px := big.NewRat(xi, 2)
					if !allows(x, px, bound.deps.HasLower1(), bound.deps.HasUpper1()) {
						continue
					}
					for yi := int64(-10); yi <= 10; yi++ {
						py := big.NewRat(yi, 2)
						if op == "inv" && px.Sign() == 0 || op == "div" && py.Sign() == 0 {
							continue
						}
						if !allows(y, py, bound.deps.HasLower2(), bound.deps.HasUpper2()) {
							continue
						}
						value := new(big.Rat)
						switch op {
						case "add":
							value.Add(px, py)
						case "sub":
							value.Sub(px, py)
						case "mul":
							value.Mul(px, py)
						case "neg":
							value.Neg(px)
						case "pow2":
							value.Mul(px, px)
						case "pow3":
							value.Mul(px, px)
							value.Mul(value, px)
						case "inv":
							value.Inv(px)
						case "div":
							value.Quo(px, py)
						}
						cmp := value.Cmp(endpoint)
						if bound.lower && (cmp < 0 || cmp == 0 && bound.open) || !bound.lower && (cmp > 0 || cmp == 0 && bound.open) {
							t.Fatalf("%s case %d x=%s y=%s deps=%d result=%s endpoint=%s lower=%t open=%t", op, i, px, py, bound.deps, value, endpoint, bound.lower, bound.open)
						}
						checks++
					}
				}
			}
		}
	}
	if checks < 10000 {
		t.Fatalf("too few implication assignments: %d", checks)
	}
}
