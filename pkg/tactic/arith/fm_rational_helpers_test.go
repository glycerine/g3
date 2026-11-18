package arith

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// fmTestNumerals is a standalone rational manager for constraints the fm
// tests build without a store.
var fmTestNumerals = func() *unumeral.RationalManager {
	b := arena.NewBudget(1 << 30)
	s, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		panic(err)
	}
	return s.Manager
}()

func fmGCD(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// fmCRat is the small rational n/d; it holds no numeral storage.
func fmCRat(n, d int64) unumeral.Rational {
	if d < 0 {
		n, d = -n, -d
	}
	if g := fmGCD(n, d); g > 1 {
		n, d = n/g, d/g
	}
	return unumeral.Rational{Num: unumeral.NewInt(int32(n)), Den: unumeral.NewInt(int32(d))}
}

func fmCRatP(n, d int64) *unumeral.Rational {
	r := fmCRat(n, d)
	return &r
}

// fmText is rational::to_string of a small rational.
func fmText(r *unumeral.Rational) string {
	if !r.IsSmall() {
		text, err := fmTestNumerals.String(r)
		if err != nil {
			panic(err)
		}
		return text
	}
	if r.Den.Val == 1 {
		return fmt.Sprint(r.Num.Val)
	}
	return fmt.Sprintf("%d/%d", r.Num.Val, r.Den.Val)
}

// fmBig is a small rational as a *big.Rat test oracle value.
func fmBig(r *unumeral.Rational) *big.Rat {
	out, ok := new(big.Rat).SetString(fmText(r))
	if !ok {
		panic(fmText(r))
	}
	return out
}

func fmRegister(t interface{ Fatal(...any) }, v *FMVariableTable, c *FMConstraint) FMRegistration {
	r, err := v.Register(c)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func fmSubsumes(t interface{ Fatal(...any) }, a, b *FMConstraint) bool {
	r, err := FMConstraintSubsumes(a, b)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
