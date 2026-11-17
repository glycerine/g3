package numeral

// Source: src/util/mpq.h add/sub (rat_add/rat_sub), mul (rat_mul), div,
// normalize, lt (rat_lt), eq, set, reset, neg, addmul/submul.
//
// mpq_manager runs these on mpz values, and mpz_manager keeps every value
// that fits an int inline (is_small): its add, sub, mul, machine_div, div,
// gcd and set then neither allocate nor emit anything in a release build.
// The functions below are those operations when every operand, every
// temporary and the result are inline. They compute in int64 first and
// change nothing unless the whole operation stays inline; otherwise the
// caller takes the full path, which also produces any cell z3 would
// allocate. The full path's local-temporary bookkeeping (the synchronized
// manager's raw scopes, the unsynchronized manager's scratch members) is
// left exactly as that path leaves it after success.

import (
	"math"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// smallRat reports inline components and a positive denominator.
func smallRat(a *Rational) bool {
	return a != nil && a.Num.Cell == 0 && a.Den.Cell == 0 && a.Den.Val > 0
}

// smallOut reports inline components (a destination's old denominator is
// not read).
func smallOut(a *Rational) bool { return a != nil && a.Num.Cell == 0 && a.Den.Cell == 0 }

// smallFastReady is checkScalarValue/check's common case plus the inline
// del admission and idle scratch: a fast operation can neither fail nor
// find a temporary that z3 would have to free.
//
// fastGate summarizes this owner's own lifecycle flags and pending scopes
// (fastGateOwner, fastGateFused: kept exact by refreshFastGate) and its raw
// local scopes (fastGateRaw: set where a raw local becomes active, cleared
// by smallFastReadySlow once none is). Admission then reads one word for
// them; the integer manager's state and the process-wide transport and
// trace-output state are read directly, since they change outside this
// owner. Checked builds compare the result with smallFastReadyFull.
func (m *RationalManager) smallFastReady() bool {
	if !smallFastBuild() || m == nil {
		return false
	}
	if m.fastGate != 0 || numericValidateCells {
		return m.smallFastReadySlow()
	}
	z := m.Z
	return m.self == m && z != nil && z.self == z && !z.closed && !z.SourceFinalizing && !z.constructionFailed &&
		!umemory.AnyFailureTransportActive() && m.RawGeneration < ^uint64(0)-4 &&
		(m.Synchronized || m.scratchInline()) && z.smallDelReady()
}

const (
	fastGateOwner uint32 = 1 << iota
	fastGateFused
	fastGateRaw
)

// smallFastReadySlow clears a stale raw-scope mark and answers from every
// flag.
func (m *RationalManager) smallFastReadySlow() bool {
	if m.fastGate&fastGateRaw != 0 && !m.RawArithmetic.Pending() {
		m.fastGate &^= fastGateRaw
	}
	full := m.smallFastReadyFull()
	if numericValidateCells && m.fastGate == 0 {
		z := m.Z
		gated := m.self == m && z != nil && z.self == z && !z.closed && !z.SourceFinalizing && !z.constructionFailed &&
			!umemory.AnyFailureTransportActive() && m.RawGeneration < ^uint64(0)-4 &&
			(m.Synchronized || m.scratchInline()) && z.smallDelReady()
		if gated != full || m.RawArithmetic.Pending() {
			panic("numeral: RationalManager fastGate out of date")
		}
	}
	return full
}

// scratchInline reports that the unsynchronized manager's scratch members
// hold no cell.
func (m *RationalManager) scratchInline() bool {
	return (m.AddTmp1.Cell | m.AddTmp2.Cell | m.NTmp.Cell | m.LtTmp1.Num.Cell | m.LtTmp1.Den.Cell |
		m.LtTmp2.Num.Cell | m.LtTmp2.Den.Cell | m.AddMulTmp.Num.Cell | m.AddMulTmp.Den.Cell) == 0
}

// smallFastReadyFull is smallFastReady computed from every flag; checked
// builds use it to verify fastGate.
func (m *RationalManager) smallFastReadyFull() bool {
	if !smallFastBuild() || !m.ownerIdle() || !m.Z.smallFastReady() || !m.Z.smallDelReady() {
		return false
	}
	if m.RawArithmetic.Pending() || m.RawGeneration >= ^uint64(0)-4 {
		return false
	}
	if m.Synchronized {
		return !m.fusedPending && !m.fusedExecuting
	}
	return m.scratchInline()
}

// ownerIdle is checkOwnerReady() == nil.
func (m *RationalManager) ownerIdle() bool {
	return m != nil && m.self == m && m.Z != nil && m.Z.self == m.Z &&
		!(m.closed || m.Closing || m.SourceFinalizing || m.SourceFinalized || m.Z.closed || m.Z.SourceFinalizing || m.Z.constructionFailed) &&
		!m.ownerFlagsBusy()
}

// ownerFlagsBusy is the part of checkOwnerReady that reads this owner's
// pending scopes.
func (m *RationalManager) ownerFlagsBusy() bool {
	return m.parseOuterPending || m.parseSuffixPending || m.parseB.pending || m.parseExp.pending || m.parseTen.pending || m.prevPowerPending || m.addIntPending || (m.fusedPending && !m.fusedExecuting) ||
		(m.hwfPending && !m.hwfExecuting)
}

// refreshFastGate recomputes fastGate after a write to one of the owner
// flags it summarizes: the lifecycle flags and pending scopes that
// checkOwnerReady reads, and, for the synchronized manager, the fused
// scope. It is Go-only bookkeeping and changes no observable state.
func (m *RationalManager) refreshFastGate() {
	g := m.fastGate & fastGateRaw
	if m.closed || m.Closing || m.SourceFinalizing || m.SourceFinalized || m.ownerFlagsBusy() {
		g |= fastGateOwner
	}
	if m.Synchronized && (m.fusedPending || m.fusedExecuting) {
		g |= fastGateFused
	}
	m.fastGate = g
}

// smallRatResult is one inline rational result: the value n/d, and for a
// rat_* path (rat true) the normalize gcd g and rat_add's temporaries.
type smallRatResult struct {
	n, d, g, t1, t2 int64
	rat             bool
}

// smallNormalize is normalize on inline n (not INT_MIN) and d > 0: the gcd,
// then exact division by it unless it is one.
func smallNormalize(r *smallRatResult) {
	r.g = smallGCD(r.n, r.d)
	if r.g != 1 {
		r.n = smallDiv(r.n, r.g)
		r.d = smallDiv(r.d, r.g)
	}
}

// smallAddCore is add/sub: the mpz path for two integers, else rat_add's
// a.num*b.den, b.num*a.den, a.den*b.den, sum and normalize.
func smallAddCore(an, ad, bn, bd int64, subtract bool) (r smallRatResult, ok bool) {
	if subtract {
		bn = -bn
	}
	if ad == 1 && bd == 1 {
		r.n, r.d = an+bn, 1
		return r, fitsInt32(r.n)
	}
	r.rat = true
	r.t1, r.t2, r.d = an*bd, bn*ad, ad*bd
	if subtract {
		// rat_sub's second temporary is b.num*a.den before the subtraction.
		r.t2 = -r.t2
		r.n = r.t1 - r.t2
	} else {
		r.n = r.t1 + r.t2
	}
	if !fitsInt32(r.t1) || !fitsInt32(r.t2) || !fitsInt32(r.d) || !fitsInt32(r.n) || r.n == math.MinInt32 {
		return r, false
	}
	smallNormalize(&r)
	return r, true
}

// smallMulCore is mul: the mpz product for two integers, else rat_mul's
// num and den products and normalize.
func smallMulCore(an, ad, bn, bd int64) (r smallRatResult, ok bool) {
	r.n, r.d = an*bn, 1
	if !fitsInt32(r.n) {
		return r, false
	}
	if ad == 1 && bd == 1 {
		return r, true
	}
	r.rat = true
	r.d = ad * bd
	if !fitsInt32(r.d) || r.n == math.MinInt32 {
		return r, false
	}
	smallNormalize(&r)
	return r, true
}

// smallCommitNormalize leaves normalize's temporary as its full path does.
func (m *RationalManager) smallCommitNormalize(g int64) {
	if !m.Synchronized {
		m.NTmp.Val = int32(g)
		return
	}
	r := &m.RawArithmetic
	m.RawGeneration++
	r.normGeneration = m.RawGeneration
	r.Norm = Int{}
	r.NormActive, r.normEnd, r.normGuard, r.normCause = false, true, false, nil
}

// smallCommitAdd leaves rat_add's temporaries as its full path does.
func (m *RationalManager) smallCommitAdd(t1, t2 int64) {
	if !m.Synchronized {
		m.AddTmp1.Val, m.AddTmp2.Val = int32(t1), int32(t2)
		return
	}
	r := &m.RawArithmetic
	m.RawGeneration++
	r.addGeneration = m.RawGeneration
	r.AddOne, r.AddTwo = Int{}, Int{}
	r.AddActive, r.addEnd, r.addGuard, r.addCursor, r.addCause = false, true, false, 2, nil
}

func (m *RationalManager) smallCommitAddResult(r *smallRatResult) {
	if r.rat {
		m.smallCommitAdd(r.t1, r.t2)
		m.smallCommitNormalize(r.g)
	}
}

func (m *RationalManager) smallCommitMulResult(r *smallRatResult) {
	if r.rat {
		m.smallCommitNormalize(r.g)
	}
}

func (m *RationalManager) addSubSmall(a, b, out *Rational, subtract bool) bool {
	if !smallRat(a) || !smallRat(b) || !smallOut(out) {
		return false
	}
	r, ok := smallAddCore(int64(a.Num.Val), int64(a.Den.Val), int64(b.Num.Val), int64(b.Den.Val), subtract)
	if !ok || !m.smallFastReady() {
		return false
	}
	m.smallCommitAddResult(&r)
	out.Num.Val, out.Den.Val = int32(r.n), int32(r.d)
	return true
}

func (m *RationalManager) mulSmall(a, b, out *Rational) bool {
	if !smallRat(a) || !smallRat(b) || !smallOut(out) {
		return false
	}
	r, ok := smallMulCore(int64(a.Num.Val), int64(a.Den.Val), int64(b.Num.Val), int64(b.Den.Val))
	if !ok || !m.smallFastReady() {
		return false
	}
	m.smallCommitMulResult(&r)
	out.Num.Val, out.Den.Val = int32(r.n), int32(r.d)
	return true
}

// divSmall is div: a.num*b.den and a.den*b.num (through the raw division
// temporary when b is the destination), the sign fix and normalize.
func (m *RationalManager) divSmall(a, b, out *Rational) bool {
	if !smallRat(a) || !smallRat(b) || !smallOut(out) || b.Num.Val == 0 {
		return false
	}
	n := int64(a.Num.Val) * int64(b.Den.Val)
	d := int64(a.Den.Val) * int64(b.Num.Val)
	if !fitsInt32(n) || !fitsInt32(d) {
		return false
	}
	if d < 0 {
		n, d = -n, -d
		if !fitsInt32(n) || !fitsInt32(d) {
			return false
		}
	}
	if n == math.MinInt32 || !m.smallFastReady() {
		return false
	}
	r := smallRatResult{n: n, d: d}
	smallNormalize(&r)
	if b == out {
		raw := &m.RawArithmetic
		m.RawGeneration++
		raw.divGeneration = m.RawGeneration
		raw.Div = Int{}
		raw.DivActive, raw.divEnd, raw.divGuard, raw.divCursor, raw.divCause = false, true, false, 1, nil
	}
	m.smallCommitNormalize(r.g)
	out.Num.Val, out.Den.Val = int32(r.n), int32(r.d)
	return true
}

func cmpInt64(x, y int64) int {
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

// compareSmall is lt's three-way form: the mpz comparison for integers,
// the sign shortcuts, then the cross products.
func (m *RationalManager) compareSmall(a, b *Rational) (int, bool) {
	if !smallRat(a) || !smallRat(b) {
		return 0, false
	}
	an, bn := int64(a.Num.Val), int64(b.Num.Val)
	if a.Den.Val == 1 && b.Den.Val == 1 {
		if !m.smallFastReady() {
			return 0, false
		}
		return cmpInt64(an, bn), true
	}
	switch {
	case an < 0 && bn >= 0, an == 0 && bn > 0:
		if !m.smallFastReady() {
			return 0, false
		}
		return -1, true
	case an == 0:
		if !m.smallFastReady() {
			return 0, false
		}
		return -cmpInt64(bn, 0), true
	case an > 0 && bn <= 0:
		if !m.smallFastReady() {
			return 0, false
		}
		return 1, true
	}
	p1, p2 := an*int64(b.Den.Val), bn*int64(a.Den.Val)
	if !fitsInt32(p1) || !fitsInt32(p2) || !m.smallFastReady() {
		return 0, false
	}
	if m.Synchronized {
		r := &m.RawArithmetic
		m.RawGeneration++
		r.compareGeneration = m.RawGeneration
		r.CompareOne, r.CompareTwo = Rational{}, Rational{}
		r.CompareActive, r.compareEnd, r.compareGuard, r.compareCursor, r.compareCause = false, true, false, 4, nil
	} else {
		m.LtTmp1.Num.Val, m.LtTmp1.Den.Val = int32(p1), 1
		m.LtTmp2.Num.Val, m.LtTmp2.Den.Val = int32(p2), 1
	}
	return cmpInt64(p1, p2), true
}

func (m *RationalManager) equalSmall(a, b *Rational) (bool, bool) {
	if !smallOut(a) || !smallOut(b) || !m.smallFastReady() {
		return false, false
	}
	return a.Num.Val == b.Num.Val && a.Den.Val == b.Den.Val, true
}

func (m *RationalManager) copySmall(dst, src *Rational) bool {
	if !smallOut(dst) || !smallOut(src) || !m.smallFastReady() {
		return false
	}
	dst.Num.Val, dst.Den.Val = src.Num.Val, src.Den.Val
	return true
}

func (m *RationalManager) resetSmall(a *Rational) bool {
	if !smallOut(a) || !m.smallFastReady() {
		return false
	}
	a.Num.Val, a.Den.Val = 0, 1
	return true
}

func (m *RationalManager) negSmall(a *Rational) bool {
	if !smallOut(a) || a.Num.Val == math.MinInt32 || !m.smallFastReady() {
		return false
	}
	a.Num.Val = -a.Num.Val
	return true
}

// fusedSmall is addmul/submul: add or sub for a unit b, else the product
// into the addmul temporary followed by add or sub.
func (m *RationalManager) fusedSmall(a, b, c, out *Rational, subtract bool) bool {
	if !smallRat(a) || !smallRat(b) || !smallRat(c) || !smallOut(out) || out.Den.Val == 0 {
		return false
	}
	if b.Den.Val == 1 && (b.Num.Val == 1 || b.Num.Val == -1) {
		return m.addSubSmall(a, c, out, subtract == (b.Num.Val == 1))
	}
	p, ok := smallMulCore(int64(b.Num.Val), int64(b.Den.Val), int64(c.Num.Val), int64(c.Den.Val))
	if !ok {
		return false
	}
	s, ok := smallAddCore(int64(a.Num.Val), int64(a.Den.Val), p.n, p.d, subtract)
	if !ok || !m.smallFastReady() {
		return false
	}
	if !m.Synchronized {
		m.AddMulTmp.Num.Val, m.AddMulTmp.Den.Val = int32(p.n), int32(p.d)
	}
	m.smallCommitMulResult(&p)
	m.smallCommitAddResult(&s)
	if m.Synchronized {
		m.fusedTemporary = Rational{}
		m.fusedPending, m.fusedExecuting, m.fusedCursor = false, false, 2
	}
	out.Num.Val, out.Den.Val = int32(s.n), int32(s.d)
	return true
}
