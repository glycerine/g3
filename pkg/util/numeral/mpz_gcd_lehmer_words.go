package numeral

// Source: src/util/mpz.cpp gcd (LEHMER_GCD), the cofactor update
// t = A*a1 + B*b1, r = C*a1 + D*b1 (mpz.cpp:798-811).

import "math/bits"

// lehmerStepWords performs the Lehmer cofactor update of gcd directly on
// the digits of a1 and b1: it publishes t = A*a1 + B*b1 and r = C*a1 +
// D*b1 into a1 and b1 (the source swaps them in), leaves the old a1 and b1
// in t and r and D in tmp, as the source mul/addmul sequence does. The
// values are the exact results of the source operations; only their
// temporaries are skipped. Like gcdWordOperands it is used only where no
// trace can observe the steps and no allocation can fail (no injected
// failure, no memory size or allocation count limit); otherwise, or when a
// result would be negative, it reports false and changes nothing.
func (m *IntegerManager) lehmerStepWords(a1, b1, t, r, tmp *Int, A, B, C, D int64) (bool, error) {
	if !m.lehmerWords || !smallFastBuild() || !m.smallFastReady() || !m.smallDelReady() || m.BudgetField.InjectionArmed() || m.Memory.AllocationLimited() {
		return false, nil
	}
	if a1.Cell == 0 || b1.Cell == 0 || a1.Val < 0 || b1.Val < 0 {
		return false, nil
	}
	// One record lookup per operand for its size and payload.
	var one [1]uint32
	opA, ok := m.craOperand(a1, &one)
	if !ok {
		return false, nil
	}
	opB, ok := m.craOperand(b1, &one)
	if !ok {
		return false, nil
	}
	x, y := opA.d, opB.d
	na := uint32(len(x))
	// The division scratch digits (contents dead between operations) hold
	// the two results.
	n := uint32(na) + 2
	if e := m.Natural.ResizeDiv(2, n); e != nil {
		return false, nil
	}
	if e := m.Natural.ResizeDiv(3, n); e != nil {
		return false, nil
	}
	tv, rv := m.Natural.Division[2][:n], m.Natural.Division[3][:n]
	if !lehmerCombine(x, y, A, B, tv) || !lehmerCombine(x, y, C, D, rv) {
		return false, nil
	}
	// mpz.cpp: set(tmp, A); mul(a1, tmp, t); set(tmp, B); addmul(t, tmp,
	// b1, t); set(tmp, C); mul(a1, tmp, r); set(tmp, D); addmul(r, tmp,
	// b1, r); swap(a1, t); swap(b1, r).
	if e := m.SetLimbs(t, tv); e != nil {
		return true, e
	}
	if e := m.SetLimbs(r, rv); e != nil {
		return true, e
	}
	if e := m.SetInt64(tmp, D); e != nil {
		return true, e
	}
	if e := m.Swap(a1, t); e != nil {
		return true, e
	}
	return true, m.Swap(b1, r)
}

// GCDDirect is GCD for a caller's coefficient loop that runs its numeral
// operations directly (see IntegerModularManager.DirectKernel): its Lehmer
// loop updates the cofactors with lehmerStepWords. The values are those of
// GCD; the cells the update allocates and frees differ.
func (m *IntegerManager) GCDDirect(a, b, c *Int) error {
	m.lehmerWords = true
	e := m.GCD(a, b, c)
	m.lehmerWords = false
	return e
}

// lehmerCombine writes P*x + Q*y into out (len(x)+2 digits, y no longer
// than x) for word cofactors |P|, |Q| <= 2^32. It reports false when the
// result is negative or does not fit.
func lehmerCombine(x, y []uint32, P, Q int64, out []uint32) bool {
	// The nonnegative-cofactor terms and the negative-cofactor terms are
	// accumulated column by column as 128-bit (hi, lo) values; out is
	// their difference.
	up, uq := uint64(P), uint64(Q)
	negP, negQ := P < 0, Q < 0
	if negP {
		up = uint64(-P)
	}
	if negQ {
		uq = uint64(-Q)
	}
	var posHi, posLo, negHi, negLo uint64
	add := func(hi, lo *uint64, c uint64, d []uint32, i int) {
		if i < len(d) && c != 0 {
			ph, pl := bits.Mul64(c, uint64(d[i]))
			var k uint64
			*lo, k = bits.Add64(*lo, pl, 0)
			*hi += ph + k
		}
	}
	var borrow uint64
	for i := range out {
		if negP {
			add(&negHi, &negLo, up, x, i)
		} else {
			add(&posHi, &posLo, up, x, i)
		}
		if negQ {
			add(&negHi, &negLo, uq, y, i)
		} else {
			add(&posHi, &posLo, uq, y, i)
		}
		d := (posLo & 0xffffffff) - (negLo & 0xffffffff) - borrow
		borrow = (d >> 63) & 1
		out[i] = uint32(d)
		posLo, posHi = posLo>>32|posHi<<32, posHi>>32
		negLo, negHi = negLo>>32|negHi<<32, negHi>>32
	}
	return borrow == 0 && posLo == 0 && posHi == 0 && negLo == 0 && negHi == 0
}
