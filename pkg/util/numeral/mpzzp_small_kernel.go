package numeral

// Source: src/util/mpzzp.h (p_normalize_core, set, mul, submul, div, inv on
// inline values); src/util/mpz.h (is_small paths); src/util/mpz.cpp rem.

import "math/bits"

// SmallModulus describes an mpzzp manager whose inline-value operations may
// be computed directly by a caller's coefficient loop (see SmallKernel).
// Z is integer mode (p_normalize does nothing). Otherwise P is an inline
// modulus, Lower/Upper are setup_p's bounds and Field is m_p_prime.
type SmallModulus struct {
	Z, Field        bool
	P, Lower, Upper int64
}

// DirectKernel reports whether a caller's coefficient loop may run m's
// operations directly on its integer manager (the Direct* methods below)
// instead of through a numeric caller context: the conditions of
// SmallKernel without requiring an inline modulus. The Direct* methods
// perform the source body (the mpz operation, then p_normalize) and
// publish the same values; they record no caller allocation roots, which
// only a failure transport reads. Like gcdWordOperands, it is off while an
// allocation can fail (an injected failure is armed, or a memory size or
// allocation count limit is set): a loop then keeps every source
// allocation, and so every source out-of-memory point and its cleanup.
func (m *IntegerModularManager) DirectKernel() bool {
	if !smallFastBuild() || m == nil || m.self != m || m.closed || m.closing {
		return false
	}
	z := m.manager
	if !z.smallFastReady() || !z.smallDelReady() || z.BudgetField.InjectionArmed() || z.Memory.AllocationLimited() {
		return false
	}
	if m.callerBindings != nil && callerBindingsCallbackActive(m.callerBindings) {
		return false
	}
	return m.z || m.primeKnown
}

// pNormalizeDirect is p_normalize_core on the integer manager.
func (m *IntegerModularManager) pNormalizeDirect(x *Int) error {
	if m.z {
		return nil
	}
	z := m.manager
	if e := z.Rem(x, &m.P, x); e != nil {
		return e
	}
	gt, e := z.Greater(x, &m.Upper)
	if e != nil {
		return e
	}
	if gt {
		return z.Sub(x, &m.P, x)
	}
	lt, e := z.Less(x, &m.Lower)
	if e != nil {
		return e
	}
	if lt {
		return z.Add(x, &m.P, x)
	}
	return nil
}

// DirectCopy is mpzzp set(a, b).
func (m *IntegerModularManager) DirectCopy(a, b *Int) error {
	one := NewInt(1)
	if done, e := m.directWordMulAdd(nil, b, &one, a, false); done || e != nil {
		return e
	}
	if e := m.manager.Copy(a, b); e != nil {
		return e
	}
	return m.pNormalizeDirect(a)
}

// DirectMul is mpzzp mul(a, b, c).
func (m *IntegerModularManager) DirectMul(a, b, c *Int) error {
	if done, e := m.directWordMulAdd(nil, a, b, c, false); done || e != nil {
		return e
	}
	if e := m.manager.Mul(a, b, c); e != nil {
		return e
	}
	return m.pNormalizeDirect(c)
}

// DirectAdd is mpzzp add(a, b, c).
func (m *IntegerModularManager) DirectAdd(a, b, c *Int) error {
	one := NewInt(1)
	if done, e := m.directWordMulAdd(a, b, &one, c, false); done || e != nil {
		return e
	}
	if e := m.manager.Add(a, b, c); e != nil {
		return e
	}
	return m.pNormalizeDirect(c)
}

// DirectSub is mpzzp sub(a, b, c).
func (m *IntegerModularManager) DirectSub(a, b, c *Int) error {
	one := NewInt(1)
	if done, e := m.directWordMulAdd(a, b, &one, c, true); done || e != nil {
		return e
	}
	if e := m.manager.Sub(a, b, c); e != nil {
		return e
	}
	return m.pNormalizeDirect(c)
}

// DirectAddMul is mpzzp addmul(a, b, c, d): d = a + b*c.
func (m *IntegerModularManager) DirectAddMul(a, b, c, d *Int) error {
	if done, e := m.directWordMulAdd(a, b, c, d, false); done || e != nil {
		return e
	}
	if e := m.manager.AddMul(a, b, c, d); e != nil {
		return e
	}
	return m.pNormalizeDirect(d)
}

// DirectAddMulTracked is DirectAddMul publishing the add-mul invocation
// receipt into child, as the caller-context operation does.
func (m *IntegerModularManager) DirectAddMulTracked(a, b, c, d *Int, child *integerAddMulInvocationReceipt) error {
	if done, e := m.directWordMulAdd(a, b, c, d, false); done || e != nil {
		return e
	}
	if e := m.manager.addMulWithAllocationContext(a, b, c, d, child, numeralContextRef(integerAllocationInvocationContext{})); e != nil {
		return e
	}
	return m.pNormalizeDirect(d)
}

// DirectSubMulTracked is DirectSubMul publishing the add-mul invocation
// receipt into child, as the caller-context operation does.
func (m *IntegerModularManager) DirectSubMulTracked(a, b, c, d *Int, child *integerAddMulInvocationReceipt) error {
	if done, e := m.directWordMulAdd(a, b, c, d, true); done || e != nil {
		return e
	}
	if e := m.manager.addSubMulWithAllocationContext(a, b, c, d, true, child, integerAllocationInvocationContext{}); e != nil {
		return e
	}
	return m.pNormalizeDirect(d)
}

// DirectSubMul is mpzzp submul(a, b, c, d): d = a - b*c.
func (m *IntegerModularManager) DirectSubMul(a, b, c, d *Int) error {
	if done, e := m.directWordMulAdd(a, b, c, d, true); done || e != nil {
		return e
	}
	if e := m.manager.SubMul(a, b, c, d); e != nil {
		return e
	}
	return m.pNormalizeDirect(d)
}

// SmallKernel reports whether inline values of m may take z3's small paths
// in a caller's loop: a release build (smallFastBuild), an open manager with
// no failure transport and no numeric or trace callback running, and an
// inline modulus in Zp mode. Every value such a loop computes equals the
// one the full operation publishes; the full operation would allocate
// nothing and emit nothing for these operands. A loop must still take the
// full operation for any big operand or result.
func (m *IntegerModularManager) SmallKernel() (SmallModulus, bool) {
	if !smallFastBuild() || m == nil || m.self != m || m.closed || m.closing {
		return SmallModulus{}, false
	}
	z := m.manager
	if !z.smallFastReady() || !z.smallDelReady() {
		return SmallModulus{}, false
	}
	if m.callerBindings != nil && callerBindingsCallbackActive(m.callerBindings) {
		return SmallModulus{}, false
	}
	if m.z {
		return SmallModulus{Z: true}, true
	}
	if !m.primeKnown || m.P.Cell != 0 || m.Lower.Cell != 0 || m.Upper.Cell != 0 || m.P.Val <= 1 {
		return SmallModulus{}, false
	}
	return SmallModulus{Field: m.prime, P: int64(m.P.Val), Lower: int64(m.Lower.Val), Upper: int64(m.Upper.Val)}, true
}

// Normalize is p_normalize on a machine value: rem (truncated, the sign of
// the dividend) and one correction into [Lower, Upper]. In Z mode it is the
// identity.
func (s SmallModulus) Normalize(v int64) int64 {
	if s.Z {
		return v
	}
	v %= s.P
	if v > s.Upper {
		v -= s.P
	} else if v < s.Lower {
		v += s.P
	}
	return v
}

// Inv is mpzzp inv on a normalized nonzero value of a prime field: the
// extended gcd cofactor, normalized. The inverse is unique, so any exact
// method yields the value z3 publishes.
func (s SmallModulus) Inv(a int64) int64 {
	r0, r1 := s.P, s.Normalize(a)
	t0, t1 := int64(0), int64(1)
	for r1 != 0 {
		q := r0 / r1
		r0, r1 = r1, r0-q*r1
		t0, t1 = t1, t0-q*t1
	}
	if r0 < 0 {
		t0 = -t0
	}
	return s.Normalize(t0)
}

// RemSmall returns rem(a, d) for d > 0: |a| mod d with the sign of a, as
// mpz rem publishes it (the remainder of a truncated division). It reads a
// big operand's digits directly and does not modify a.
func (m *IntegerManager) RemSmall(a *Int, d uint32) int64 {
	if a.Cell == 0 {
		return int64(a.Val) % int64(d)
	}
	c := m.Cells.recordPtr(NumericSlot(a.Cell))
	if c == nil {
		panic(ErrNumericOwner)
	}
	digits := c.Backing.digitsUnchecked(m.Allocator)[:c.Size]
	var r uint64
	dd := uint64(d)
	for i := len(digits) - 1; i >= 0; i-- {
		r = (r<<32 | uint64(digits[i])) % dd
	}
	if a.Val < 0 {
		return -int64(r)
	}
	return int64(r)
}

// wordValue reads a value of magnitude below 2^63 (an inline value or a
// cell of at most two digits) without modifying it.
func (m *IntegerManager) wordValue(a *Int) (int64, bool) {
	if a.Cell == 0 {
		return int64(a.Val), true
	}
	c := m.Cells.recordPtr(NumericSlot(a.Cell))
	if c == nil || c.Size > 2 {
		return 0, false
	}
	d := c.Backing.digitsUnchecked(m.Allocator)
	var v uint64
	switch c.Size {
	case 2:
		v = uint64(d[1])<<32 | uint64(d[0])
	case 1:
		v = uint64(d[0])
	}
	if v>>63 != 0 {
		return 0, false
	}
	if a.Val < 0 {
		return -int64(v), true
	}
	return int64(v), true
}

// wordModulus returns p when m is in Zp mode with p below 2^62.
func (m *IntegerModularManager) wordModulus() (uint64, bool) {
	if m.z {
		return 0, false
	}
	p, ok := m.manager.wordValue(&m.P)
	if !ok || p <= 1 || p >= 1<<62 {
		return 0, false
	}
	return uint64(p), true
}

// wordResidue is v mod p in [0, p).
func wordResidue(v int64, p uint64) uint64 {
	if v >= 0 {
		return uint64(v) % p
	}
	r := (uint64(-v)) % p
	if r == 0 {
		return 0
	}
	return p - r
}

// wordProductResidue is b*c mod p in [0, p) for |b|, |c| < 2^63.
func wordProductResidue(b, c int64, p uint64) uint64 {
	neg := (b < 0) != (c < 0)
	ub, uc := uint64(b), uint64(c)
	if b < 0 {
		ub = uint64(-b)
	}
	if c < 0 {
		uc = uint64(-c)
	}
	hi, lo := bits.Mul64(ub, uc)
	_, r := bits.Div64(hi%p, lo, p)
	if neg && r != 0 {
		r = p - r
	}
	return r
}

// wordPublish stores the p_normalize result of residue r in [0, p): the
// unique value congruent to r in [Lower, Upper] (setup_p's bounds, the
// range p_normalize_core maps every remainder into), set as set_i64 does.
func (m *IntegerModularManager) wordPublish(d *Int, r, p uint64) error {
	v := int64(r)
	if r > p/2 {
		v = int64(r) - int64(p)
	}
	return m.manager.SetInt64(d, v)
}

// directWordMulAdd computes d = a + b*c (a - b*c when subtract; a nil a
// is zero) and p_normalize for a modulus below 2^62 and operands of
// magnitude below 2^63 with machine arithmetic. The normalized result is
// the unique value of its residue class in [Lower, Upper], whatever
// intermediate z3's mpz operations take. It reports false, changing
// nothing, when the modulus or an operand is not such a word value.
func (m *IntegerModularManager) directWordMulAdd(a, b, c, d *Int, subtract bool) (bool, error) {
	p, ok := m.wordModulus()
	if !ok {
		return false, nil
	}
	z := m.manager
	var av int64
	if a != nil {
		if av, ok = z.wordValue(a); !ok {
			return false, nil
		}
	}
	bv, ok := z.wordValue(b)
	if !ok {
		return false, nil
	}
	cv, ok := z.wordValue(c)
	if !ok {
		return false, nil
	}
	prod := wordProductResidue(bv, cv, p)
	if subtract && prod != 0 {
		prod = p - prod
	}
	r := wordResidue(av, p) + prod
	if r >= p {
		r -= p
	}
	return true, m.wordPublish(d, r, p)
}
