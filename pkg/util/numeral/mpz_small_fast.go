package numeral

// Source: src/util/mpz.h (is_small fast paths of set, add, sub, mul,
// machine_div, div, machine_div_rem and set_i64); src/util/mpz.cpp gcd.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// smallFastBuild reports a build in which small values take the inline
// paths below. z3's small paths emit nothing in a release build (STRACE is
// compiled out) and allocate nothing; g3trace builds keep the full paths
// so every trace record is emitted where z3 emits it, and g3debug builds
// keep them so every source assertion runs.
func smallFastBuild() bool { return !diagnostic.TraceCompiled() && !diagnostic.DebugBuild }

// smallFastReady is check's common case for an inline operand: an open
// manager with no failure transport running. Any other state takes the
// full path, which returns the same error in the same order.
func (m *IntegerManager) smallFastReady() bool {
	return m != nil && m.self == m && !umemory.AnyFailureTransportActive() && !m.closed && !m.SourceFinalizing && !m.constructionFailed
}

// smallDelReady is del's cleanup admission for an inline value (see
// checkCleanupWithModularOwner) once smallFastReady holds: no numeric
// caller callback and no trace output callback is executing.
func (m *IntegerManager) smallDelReady() bool {
	return (m.callerBindings == nil || !callerBindingsCallbackActive(m.callerBindings)) && !m.sourceTraceTags().OutputCallbackActive()
}

func fitsInt32(v int64) bool { return v >= math.MinInt32 && v <= math.MaxInt32 }

// smallInts reports three inline operands (nil-safe).
func smallInts(a, b, c *Int) bool {
	return a != nil && b != nil && c != nil && a.Cell == 0 && b.Cell == 0 && c.Cell == 0
}

// smallGCD is u_gcd on |a|, |b| for inline values other than INT_MIN (the
// operands z3's gcd handles on its small path).
func smallGCD(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	return int64(U32GCD(uint32(a), uint32(b)))
}

// smallDiv is mpz div on inline values with a nonzero divisor: machine
// division, adjusted toward -infinity for a negative dividend with a
// nonzero remainder (machine_div_rem's tmp).
func smallDiv(a, b int64) int64 {
	q := a / b
	if a < 0 && a%b != 0 {
		if b < 0 {
			q++
		} else {
			q--
		}
	}
	return q
}

// copySmall is set(target, source) for an inline source and target: del of
// an inline target is a no-op, then the value is stored.
func (m *IntegerManager) copySmall(dst, src *Int) bool {
	if !smallFastBuild() || dst == nil || src == nil || dst.Cell != 0 || src.Cell != 0 || !m.smallFastReady() || !m.smallDelReady() {
		return false
	}
	dst.Val = src.Val
	return true
}

// divSmall is div's small path when the quotient is inline.
func (m *IntegerManager) divSmall(a, b, c *Int) bool {
	// c == b with a negative dividend rereads b after publishing the
	// quotient (see Div); that alias takes the full path.
	if !smallFastBuild() || !smallInts(a, b, c) || b.Val == 0 || (a.Val < 0 && c == b) {
		return false
	}
	q := smallDiv(int64(a.Val), int64(b.Val))
	if !fitsInt32(q) || !m.smallFastReady() || !m.smallDelReady() {
		return false
	}
	c.Val = int32(q)
	return true
}

// gcdSmall is gcd's small path (neither operand INT_MIN).
func (m *IntegerManager) gcdSmall(a, b, c *Int) bool {
	if !smallFastBuild() || !smallInts(a, b, c) || a.Val == math.MinInt32 || b.Val == math.MinInt32 || !m.smallFastReady() || !m.smallDelReady() {
		return false
	}
	c.Val = int32(smallGCD(int64(a.Val), int64(b.Val)))
	return true
}
