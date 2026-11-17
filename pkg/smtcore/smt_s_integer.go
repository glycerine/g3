package smtcore

// Source: src/util/s_integer.h and src/util/inf_s_integer.h: the machine
// integer numerals of the dense difference-logic instantiations si_ext
// (s_integer) and smi_ext (inf_s_integer).
//
// z3's s_integer wraps a C++ int (m_val) and inf_s_integer two of them
// (m_first, m_second). Their +=, -=, neg and the rational constructor do no
// range check: a signed overflow is undefined behavior, and
// static_cast<int>(r.get_int64()) of a rational outside the int range (or
// not an integer) has no meaningful value. g3 never wraps silently: each
// operation checks the int32 range and panics where the C++ result would be
// undefined. setup selects these instantiations only when
// static_features::arith_k_sum_is_small() (the absolute numerals of the
// asserted atoms sum below INT_MAX/8), so a distance, a sum of at most three
// shortest-path segments, stays below INT_MAX for every input the setup
// admits; the features are collected from every formula of the check that
// configures the context (setup_and_check), and incremental checks configure
// by logic only (no dense theory).

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtSInteger is s_integer: int m_val.
type smtSInteger struct{ val int32 }

// smtSIntegerOverflow panics for a C++ signed int overflow (undefined
// behavior in s_integer / inf_s_integer arithmetic).
func smtSIntegerOverflow(op string, a, b int64) {
	panic(fmt.Sprintf("smtcore: s_integer %s overflows int (%d, %d): undefined behavior in z3's s_integer/inf_s_integer (src/util/s_integer.h, inf_s_integer.h)", op, a, b))
}

func smtSIntegerAdd32(a, b int32) int32 {
	s := int64(a) + int64(b)
	if s < math.MinInt32 || s > math.MaxInt32 {
		smtSIntegerOverflow("+", int64(a), int64(b))
	}
	return int32(s)
}

func smtSIntegerSub32(a, b int32) int32 {
	s := int64(a) - int64(b)
	if s < math.MinInt32 || s > math.MaxInt32 {
		smtSIntegerOverflow("-", int64(a), int64(b))
	}
	return int32(s)
}

func smtSIntegerNeg32(a int32) int32 {
	if a == math.MinInt32 {
		smtSIntegerOverflow("neg", int64(a), 0)
	}
	return -a
}

// smtSIntegerOfRational is static_cast<int>(r.get_int64()), the s_integer
// and inf_s_integer rational constructors. get_int64 requires an int64
// integer (SASSERT) and the cast an int value; outside that the C++ value is
// meaningless, so g3 panics.
func smtSIntegerOfRational(q *numeral.RationalManager, r *numeral.Rational) int32 {
	if r.IsSmall() && r.Den.Val == 1 {
		return r.Num.Val
	}
	var b big.Rat
	if err := q.BigRatOf(r, &b); err != nil {
		panic(fmt.Sprintf("smtcore: s_integer(rational): %v", err))
	}
	if b.IsInt() && b.Num().IsInt64() {
		if v := b.Num().Int64(); v >= math.MinInt32 && v <= math.MaxInt32 {
			return int32(v)
		}
	}
	panic(fmt.Sprintf("smtcore: s_integer(rational %s) is not an int: static_cast<int>(r.get_int64()) has no defined value in z3 (src/util/s_integer.h)", b.RatString()))
}

func (a smtSInteger) add(b smtSInteger) smtSInteger {
	return smtSInteger{smtSIntegerAdd32(a.val, b.val)}
}
func (a smtSInteger) sub(b smtSInteger) smtSInteger {
	return smtSInteger{smtSIntegerSub32(a.val, b.val)}
}
func (a smtSInteger) neg() smtSInteger { return smtSInteger{smtSIntegerNeg32(a.val)} }
func (a smtSInteger) String() string   { return strconv.Itoa(int(a.val)) }

// smtInfSInteger is inf_s_integer: m_first + m_second*epsilon.
type smtInfSInteger struct{ first, second int32 }

func (a smtInfSInteger) add(b smtInfSInteger) smtInfSInteger {
	return smtInfSInteger{smtSIntegerAdd32(a.first, b.first), smtSIntegerAdd32(a.second, b.second)}
}

func (a smtInfSInteger) sub(b smtInfSInteger) smtInfSInteger {
	return smtInfSInteger{smtSIntegerSub32(a.first, b.first), smtSIntegerSub32(a.second, b.second)}
}

func (a smtInfSInteger) neg() smtInfSInteger {
	return smtInfSInteger{smtSIntegerNeg32(a.first), smtSIntegerNeg32(a.second)}
}

// less is operator<(inf_s_integer, inf_s_integer).
func (a smtInfSInteger) less(b smtInfSInteger) bool {
	return a.first < b.first || a.first == b.first && a.second < b.second
}

func (a smtInfSInteger) isZero() bool { return a.first == 0 && a.second == 0 }

// String is operator<<(ostream, inf_s_integer): the infinitesimal is printed
// with its sign, after " -e*" when it is negative.
func (a smtInfSInteger) String() string {
	switch {
	case a.second == 0:
		return strconv.Itoa(int(a.first))
	case a.second < 0:
		return "(" + strconv.Itoa(int(a.first)) + " -e*" + strconv.Itoa(int(a.second)) + ")"
	}
	return "(" + strconv.Itoa(int(a.first)) + " +e*" + strconv.Itoa(int(a.second)) + ")"
}

// widthText is out.width(w); out << std::left << a: the width applies to the
// first insertion only, which is "(" when the infinitesimal is not zero.
func (a smtInfSInteger) widthText(w int) string {
	if a.second == 0 {
		return fmt.Sprintf("%-*d", w, a.first)
	}
	text := a.String()
	return fmt.Sprintf("%-*s", w, "(") + text[1:]
}
