package smtcore

// Exact numerals of the dense difference-logic matrix (the numeral of
// theory_dense_diff_logic<Ext>, inf_numeral r + e*epsilon). Not a port of a
// source type: the C++ instantiations use machine integers (smi_ext) or
// rationals with a small-integer fast path; this keeps every value exact,
// in int64 while both parts are integers that fit, otherwise in a shared,
// never-mutated smtInfRational. Results equal smtInfRational arithmetic.

import (
	"math"
	"math/big"
)

// smtInfRational is the dense matrix exact fallback value (moved from the
// diff-logic graph, which now uses smtInfIntRational).
// smtInfRational represents a + b*epsilon, ordered lexicographically with a
// positive infinitesimal epsilon. Values own their big.Rat components.
type smtInfRational struct {
	rational, infinitesimal big.Rat
}

func newSMTInfRational(rational, infinitesimal *big.Rat) smtInfRational {
	var result smtInfRational
	if rational != nil {
		result.rational.Set(rational)
	}
	if infinitesimal != nil {
		result.infinitesimal.Set(infinitesimal)
	}
	return result
}

func (v *smtInfRational) Set(other *smtInfRational) *smtInfRational {
	v.rational.Set(&other.rational)
	v.infinitesimal.Set(&other.infinitesimal)
	return v
}

func (v *smtInfRational) Add(left, right *smtInfRational) *smtInfRational {
	smtRatAddSub(&v.rational, &left.rational, &right.rational, false)
	smtRatAddSub(&v.infinitesimal, &left.infinitesimal, &right.infinitesimal, false)
	return v
}

func (v *smtInfRational) Sub(left, right *smtInfRational) *smtInfRational {
	smtRatAddSub(&v.rational, &left.rational, &right.rational, true)
	smtRatAddSub(&v.infinitesimal, &left.infinitesimal, &right.infinitesimal, true)
	return v
}

// smtRatAddSub is big.Rat.Add (or Sub). When the destination and both
// operands are integral, it adds the numerators in place: the denominator
// stays 1, and Rat's scaling, normalization and temporaries are skipped.
func smtRatAddSub(z, x, y *big.Rat, subtract bool) {
	if z.IsInt() && x.IsInt() && y.IsInt() {
		if subtract {
			z.Num().Sub(x.Num(), y.Num())
		} else {
			z.Num().Add(x.Num(), y.Num())
		}
		return
	}
	if subtract {
		z.Sub(x, y)
	} else {
		z.Add(x, y)
	}
}

func (v *smtInfRational) Neg(other *smtInfRational) *smtInfRational {
	v.rational.Neg(&other.rational)
	v.infinitesimal.Neg(&other.infinitesimal)
	return v
}

func (v *smtInfRational) Cmp(other *smtInfRational) int {
	if result := smtRatCmp(&v.rational, &other.rational); result != 0 {
		return result
	}
	return smtRatCmp(&v.infinitesimal, &other.infinitesimal)
}

func (v *smtInfRational) Sign() int {
	var zero smtInfRational
	return v.Cmp(&zero)
}

type smtDenseNum struct {
	r, e int64           // r + e*epsilon, when x is nil
	x    *smtInfRational // exact value otherwise; never mutated after creation
}

// smtDenseNumOf converts an exact value, keeping it small when it can.
func smtDenseNumOf(v *smtInfRational) smtDenseNum {
	if v.rational.IsInt() && v.infinitesimal.IsInt() {
		r, e := v.rational.Num(), v.infinitesimal.Num()
		if r.IsInt64() && e.IsInt64() {
			return smtDenseNum{r: r.Int64(), e: e.Int64()}
		}
	}
	c := new(smtInfRational)
	c.Set(v)
	return smtDenseNum{x: c}
}

// inf returns the value as a fresh smtInfRational.
func (a smtDenseNum) inf() smtInfRational {
	var v smtInfRational
	if a.x != nil {
		v.Set(a.x)
		return v
	}
	v.rational.SetInt64(a.r)
	v.infinitesimal.SetInt64(a.e)
	return v
}

func smtDenseAdd64(a, b int64) (int64, bool) {
	s := a + b
	return s, (a >= 0) != (b >= 0) || (s >= 0) == (a >= 0)
}

func smtDenseNumAdd(a, b smtDenseNum) smtDenseNum {
	if a.x == nil && b.x == nil {
		r, ok := smtDenseAdd64(a.r, b.r)
		e, ok2 := smtDenseAdd64(a.e, b.e)
		if ok && ok2 {
			return smtDenseNum{r: r, e: e}
		}
	}
	left, right := a.inf(), b.inf()
	var sum smtInfRational
	sum.Add(&left, &right)
	return smtDenseNumOf(&sum)
}

func smtDenseNumNeg(a smtDenseNum) smtDenseNum {
	if a.x == nil && a.r != math.MinInt64 && a.e != math.MinInt64 {
		return smtDenseNum{r: -a.r, e: -a.e}
	}
	v := a.inf()
	v.Neg(&v)
	return smtDenseNumOf(&v)
}

func smtDenseCmp64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// smtDenseNumCmp is the lexicographic order of r + e*epsilon.
func smtDenseNumCmp(a, b smtDenseNum) int {
	if a.x == nil && b.x == nil {
		if c := smtDenseCmp64(a.r, b.r); c != 0 {
			return c
		}
		return smtDenseCmp64(a.e, b.e)
	}
	left, right := a.inf(), b.inf()
	return left.Cmp(&right)
}

// smtDenseNumText is smtDenseInfText of the value.
func smtDenseNumText(a smtDenseNum) string {
	v := a.inf()
	return smtDenseInfText(&v)
}
