package numeral

// Source: src/util/hwf.cpp:222-255,586-650.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The selected Linux set_rounding_mode switch has no nearest-away or unknown
// mode. A debug Continue leaves the prior process rounding mode in place.
// The current port is single-goroutine, so one selected mode represents that
// process setting without changing Go's host FPU environment.
var hwfSourceSelectedMode = ArbitraryFloatNearestEven

func hwfSourceRoundingMode(mode ArbitraryFloatRoundingMode) (ArbitraryFloatRoundingMode, error) {
	switch mode {
	case ArbitraryFloatNearestEven, ArbitraryFloatTowardPositive, ArbitraryFloatTowardNegative, ArbitraryFloatTowardZero:
		hwfSourceSelectedMode = mode
		return mode, nil
	default:
		if e := diagnostic.BorrowProcessDebugContext().UnreachableSource("src/util/hwf.cpp", 642); e != nil {
			return mode, e
		}
		return hwfSourceSelectedMode, nil
	}
}

// The source sets the process rounding mode and lets the FPU round. Go
// arithmetic always rounds to nearest-even, so each operation computes the
// nearest result natively and then the sign of the exact rounding error from
// error-free double transformations; a directed mode moves the nearest
// result one step when the error points the other way. This replaces the
// host FPU setting without any arbitrary-precision arithmetic.

// hwfFrexp splits a finite nonzero x into f in [0.5,1) and an exponent.
func hwfFrexp(x float64) (float64, int) { return math.Frexp(math.Abs(x)) }

// hwfErrorSign is the sign of exact - nearest for one finite operation on
// finite operands whose exact result is nonzero and nearest is finite.
func hwfErrorSign(op byte, a, b, nearest float64) int {
	switch op {
	case '+', '-':
		if op == '-' {
			b = -b
		}
		// Fast2Sum with |a| >= |b|: nearest - a is exact and so is the
		// error b - (nearest - a), and neither step can overflow.
		if math.Abs(a) < math.Abs(b) {
			a, b = b, a
		}
		return hwfSign(float64(b - float64(nearest-a)))
	case '*':
		sign := 1
		if math.Signbit(a) != math.Signbit(b) {
			sign = -1
		}
		if nearest == 0 {
			return sign
		}
		// |a*b| - |nearest| on normalized significands, where the product
		// is exact as a Dekker pair and the difference an exact expansion.
		fa, ea := hwfFrexp(a)
		fb, eb := hwfFrexp(b)
		fp, ep := hwfFrexp(nearest)
		hi, lo := hwfTwoProduct(fa, fb)
		return sign * hwfExpansionSign(lo, hi, -math.Ldexp(fp, ep-ea-eb))
	case '/':
		sign := 1
		if math.Signbit(a) != math.Signbit(b) {
			sign = -1
		}
		if nearest == 0 {
			return sign
		}
		// |a| - |nearest|*|b| has the sign of |a/b| - |nearest|.
		fa, ea := hwfFrexp(a)
		fb, eb := hwfFrexp(b)
		fq, eq := hwfFrexp(nearest)
		hi, lo := hwfTwoProduct(math.Ldexp(fq, eq+eb-ea), fb)
		return sign * hwfExpansionSign(-lo, -hi, fa)
	}
	return 0
}

// hwfTwoProduct is Dekker's exact product x*y = hi + lo with Veltkamp
// splitting, valid without overflow or underflow; every product is rounded
// explicitly so that no compiler fuses it (z3's x86-64 build has no FMA).
func hwfTwoProduct(x, y float64) (hi, lo float64) {
	hi = float64(x * y)
	xh, xl := hwfSplit(x)
	yh, yl := hwfSplit(y)
	lo = float64(float64(float64(float64(float64(xh*yh)-hi)+float64(xh*yl))+float64(xl*yh)) + float64(xl*yl))
	return hi, lo
}

func hwfSplit(x float64) (hi, lo float64) {
	c := float64(134217729 * x) // 2^27 + 1
	hi = float64(c - float64(c-x))
	return hi, float64(x - hi)
}

// hwfTwoSum is Knuth's exact sum a + b = s + e.
func hwfTwoSum(a, b float64) (s, e float64) {
	s = float64(a + b)
	bb := float64(s - a)
	return s, float64(float64(a-float64(s-bb)) + float64(b-bb))
}

// hwfExpansionSign is the exact sign of lo + hi + x, where lo and hi are a
// nonoverlapping pair with |lo| <= |hi|: Shewchuk's grow-expansion gives a
// nonoverlapping expansion of increasing magnitude whose largest nonzero
// component carries the sign of the sum.
func hwfExpansionSign(lo, hi, x float64) int {
	q, h0 := hwfTwoSum(x, lo)
	q, h1 := hwfTwoSum(q, hi)
	if q != 0 {
		return hwfSign(q)
	}
	if h1 != 0 {
		return hwfSign(h1)
	}
	return hwfSign(h0)
}

func hwfSign(x float64) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

// hwfRound gives the directed result from the nearest one and the sign of
// exact - nearest, with the source's signed zero for exact zero results.
func hwfRound(nearest float64, errorSign, exactSign int, mode ArbitraryFloatRoundingMode, negativeZero bool) float64 {
	if exactSign == 0 {
		if negativeZero {
			return math.Copysign(0, -1)
		}
		return 0
	}
	if errorSign == 0 || mode == ArbitraryFloatNearestEven {
		return nearest
	}
	if (mode == ArbitraryFloatTowardPositive || mode == ArbitraryFloatTowardZero && exactSign < 0) && errorSign > 0 {
		return math.Nextafter(nearest, math.Inf(1))
	}
	if (mode == ArbitraryFloatTowardNegative || mode == ArbitraryFloatTowardZero && exactSign > 0) && errorSign < 0 {
		return math.Nextafter(nearest, math.Inf(-1))
	}
	return nearest
}

func (m *HardwareFloatManager) arithmetic(mode ArbitraryFloatRoundingMode, x, y, out *HardwareFloat, op byte) error {
	if m == nil || x == nil || y == nil || out == nil {
		return ErrNumericOwner
	}
	mode, e := hwfSourceRoundingMode(mode)
	if e != nil {
		return e
	}
	a, b := x.value, y.value
	var nearest float64
	switch op {
	case '+':
		nearest = float64(a + b)
	case '-':
		nearest = float64(a - b)
	case '*':
		nearest = float64(a * b)
	case '/':
		nearest = float64(a / b)
	}
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) || op == '/' && b == 0 {
		out.value = nearest
		return nil
	}
	exactSign := 0
	negativeZero := false
	switch op {
	case '+', '-':
		// A finite sum rounds to zero only when it is exactly zero.
		if nearest != 0 {
			exactSign = hwfSign(nearest)
		} else if op == '+' {
			negativeZero = (a == 0 && b == 0 && math.Signbit(a) && math.Signbit(b)) || (mode == ArbitraryFloatTowardNegative && (a != 0 || b != 0 || math.Signbit(a) != math.Signbit(b)))
		} else {
			negativeZero = (a == 0 && b == 0 && math.Signbit(a) && !math.Signbit(b)) || (mode == ArbitraryFloatTowardNegative && (a != 0 || b != 0 || math.Signbit(a) == math.Signbit(b)))
		}
	case '*', '/':
		if a != 0 && b != 0 {
			exactSign = 1
			if math.Signbit(a) != math.Signbit(b) {
				exactSign = -1
			}
		}
		negativeZero = math.Signbit(a) != math.Signbit(b)
	}
	if exactSign == 0 {
		out.value = hwfRound(nearest, 0, 0, mode, negativeZero)
		return nil
	}
	var errorSign int
	switch {
	case math.IsInf(nearest, 1):
		errorSign = -1
	case math.IsInf(nearest, -1):
		errorSign = 1
	default:
		errorSign = hwfErrorSign(op, a, b, nearest)
	}
	out.value = hwfRound(nearest, errorSign, exactSign, mode, negativeZero)
	return nil
}

func (m *HardwareFloatManager) Add(mode ArbitraryFloatRoundingMode, x, y, out *HardwareFloat) error {
	return m.arithmetic(mode, x, y, out, '+')
}
func (m *HardwareFloatManager) Sub(mode ArbitraryFloatRoundingMode, x, y, out *HardwareFloat) error {
	return m.arithmetic(mode, x, y, out, '-')
}
func (m *HardwareFloatManager) Mul(mode ArbitraryFloatRoundingMode, x, y, out *HardwareFloat) error {
	return m.arithmetic(mode, x, y, out, '*')
}
func (m *HardwareFloatManager) Div(mode ArbitraryFloatRoundingMode, x, y, out *HardwareFloat) error {
	return m.arithmetic(mode, x, y, out, '/')
}
