package smtcore

// Exact numerals of the dense difference-logic matrix (the numeral of
// theory_dense_diff_logic<Ext>, inf_numeral r + e*epsilon). Not a port of a
// source type: the C++ instantiations use machine integers (smi_ext) or
// rationals with a small-integer fast path; this keeps every value exact,
// in int64 while both parts are integers that fit, otherwise in a shared,
// never-mutated smtInfRational. Results equal smtInfRational arithmetic.

import "math"

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
