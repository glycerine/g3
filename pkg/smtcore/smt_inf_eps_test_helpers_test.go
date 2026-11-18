package smtcore

// Test-only readers and builders of small inf_eps values. The values the
// tests build and check are small (inline) rationals, which belong to no
// manager's cells, so they need no manager here; a big value is a test bug.

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smallRational is x as an inline rational.
func smallRational(x *big.Rat) numeral.Rational {
	if x == nil {
		return numeral.NewRational()
	}
	if !x.Num().IsInt64() || !x.Denom().IsInt64() {
		panic(fmt.Sprintf("test value %s is not small", x.RatString()))
	}
	n, d := x.Num().Int64(), x.Denom().Int64()
	if int64(int32(n)) != n || int64(int32(d)) != d {
		panic(fmt.Sprintf("test value %s is not small", x.RatString()))
	}
	return numeral.Rational{Num: numeral.NewInt(int32(n)), Den: numeral.NewInt(int32(d))}
}

// smallBig reads an inline rational.
func smallBig(r *numeral.Rational) *big.Rat {
	if !r.IsSmall() {
		panic("test helper read a big rational")
	}
	return big.NewRat(int64(r.Num.Val), int64(r.Den.Val))
}

// setSmall sets an inline rational to n.
func setSmall(r *numeral.Rational, n int64) { *r = smallRational(big.NewRat(n, 1)) }

func testNII(infinity, main, epsilon *big.Rat) simplex.Infinitesimal {
	return simplex.Infinitesimal{Infinity: smallRational(infinity), Main: smallRational(main), Epsilon: smallRational(epsilon)}
}
func testNI(main, epsilon *big.Rat) simplex.Infinitesimal { return testNII(nil, main, epsilon) }
func testRI(main *big.Rat) simplex.Infinitesimal          { return testNII(nil, main, nil) }

func testObjectiveValue() SMTArithmeticObjectiveValue {
	return SMTArithmeticObjectiveValue{Finite: simplex.ZeroInfinitesimal()}
}

// bigInf is an inf_eps value read into math/big, for comparisons.
type bigInf struct{ Infinity, Main, Epsilon big.Rat }

func asBigInf(v *simplex.Infinitesimal) *bigInf {
	var r bigInf
	r.Infinity.Set(smallBig(&v.Infinity))
	r.Main.Set(smallBig(&v.Main))
	r.Epsilon.Set(smallBig(&v.Epsilon))
	return &r
}

func (a *bigInf) Cmp(b *bigInf) int {
	if n := a.Infinity.Cmp(&b.Infinity); n != 0 {
		return n
	}
	if n := a.Main.Cmp(&b.Main); n != 0 {
		return n
	}
	return a.Epsilon.Cmp(&b.Epsilon)
}

func (a *bigInf) IsRational() bool { return a.Infinity.Sign() == 0 && a.Epsilon.Sign() == 0 }

func (a *bigInf) String() string {
	text := a.Main.RatString()
	if a.Epsilon.Sign() != 0 {
		text += "+" + a.Epsilon.RatString() + "*epsilon"
	}
	if a.Infinity.Sign() != 0 {
		text = a.Infinity.RatString() + "*infinity+" + text
	}
	return text
}

// infCmp compares two small inf_eps values.
func infCmp(a, b *simplex.Infinitesimal) int { return asBigInf(a).Cmp(asBigInf(b)) }
