package numeral

// Source: src/math/polynomial/upolynomial.cpp:694-703 (CRA_combine_images
// ADD: mul, mul, add, mod, gt, sub) over src/util/mpz.cpp (big_mul,
// quot_rem_core, mod) and src/util/mpn.cpp (mul, div).

// CRAScratch holds a caller's Go digit buffers for CRACoefficient, reused
// across the coefficients of one image combination.
type CRAScratch struct {
	p1, p2, sum, q, r []uint32
	// one holds the magnitudes of inline operands (see craOperand).
	one [6][1]uint32
}

// craDigits is a value as a sign and its magnitude digits (no leading zero
// digits; zero has none). An inline value's magnitude is held in one.
type craDigits struct {
	neg bool
	d   []uint32
}

func (m *IntegerManager) craOperand(a *Int, one *[1]uint32) (craDigits, bool) {
	if a.Cell == 0 {
		v := int64(a.Val)
		if v == 0 {
			return craDigits{}, true
		}
		neg := v < 0
		if neg {
			v = -v
		}
		one[0] = uint32(v)
		return craDigits{neg: neg, d: one[:]}, true
	}
	if numericValidateCells {
		return craDigits{}, false
	}
	// One record lookup for the size and the payload (digitsDirect's).
	c := m.Cells.recordPtr(NumericSlot(a.Cell))
	if c == nil {
		return craDigits{}, false
	}
	d, limit, n := c.Backing.digitsUnchecked(m.Allocator), max(c.Size, c.span), c.Size
	if n > limit || int(n) > len(d) || n == 0 || d[n-1] == 0 {
		return craDigits{}, false
	}
	return craDigits{neg: a.Val < 0, d: d[:n]}, true
}

func craGrow(b []uint32, n int) []uint32 {
	if cap(b) < n {
		return make([]uint32, n, n+n/2)
	}
	return b[:n]
}

func craTrim(d []uint32) []uint32 {
	for len(d) > 0 && d[len(d)-1] == 0 {
		d = d[:len(d)-1]
	}
	return d
}

func craCompare(a, b []uint32) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// craMul is the magnitude product into buf.
func craMul(a, b []uint32, buf []uint32) []uint32 {
	if len(a) == 0 || len(b) == 0 {
		return buf[:0]
	}
	c := craGrow(buf, len(a)+len(b))
	multiplyDigits(a, uint32(len(a)), uint32(len(a)), b, uint32(len(b)), uint32(len(b)), c, uint32(len(c)))
	return craTrim(c)
}

// craAddSub returns the signed sum of x and y into buf (magnitudes trimmed).
func craAddSub(x, y craDigits, buf []uint32) craDigits {
	if len(y.d) > len(x.d) || (len(y.d) == len(x.d) && craCompare(y.d, x.d) > 0) {
		x, y = y, x
	}
	out := craGrow(buf, len(x.d)+1)
	if x.neg == y.neg {
		var k uint32
		for i := range x.d {
			var v uint32
			if i < len(y.d) {
				v = y.d[i]
			}
			s := uint64(x.d[i]) + uint64(v) + uint64(k)
			out[i] = uint32(s)
			k = uint32(s >> 32)
		}
		out[len(x.d)] = k
		return craDigits{neg: x.neg, d: craTrim(out)}
	}
	// |x| >= |y|: the result takes x's sign.
	var borrow uint64
	for i := range x.d {
		var v uint32
		if i < len(y.d) {
			v = y.d[i]
		}
		s := uint64(x.d[i]) - uint64(v) - borrow
		borrow = (s >> 63) & 1
		out[i] = uint32(s)
	}
	out[len(x.d)] = 0
	r := craTrim(out)
	return craDigits{neg: x.neg && len(r) > 0, d: r}
}

// CRACoefficient computes one coefficient of CRA_combine_images directly
// on digits: v = x*a1 + y*a2; new_a = mod(v, bound); if new_a > upper
// then new_a -= bound; and publishes new_a into out as set does. bound
// must be positive and upper its half. The value is the exact result of
// the source mpz operations; their temporaries (tmp1..tmp3) are not
// published. The caller runs it only under IntegerModularManager's
// DirectKernel conditions; it reports false, changing nothing, for an
// operand it does not read directly.
func (m *IntegerManager) CRACoefficient(x, y, a1, a2, bound, upper, out *Int, s *CRAScratch) (bool, error) {
	X, ok1 := m.craOperand(x, &s.one[0])
	Y, ok2 := m.craOperand(y, &s.one[1])
	A1, ok3 := m.craOperand(a1, &s.one[2])
	A2, ok4 := m.craOperand(a2, &s.one[3])
	B, ok5 := m.craOperand(bound, &s.one[4])
	U, ok6 := m.craOperand(upper, &s.one[5])
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6) || len(B.d) == 0 || B.neg || U.neg {
		return false, nil
	}
	s.p1 = craMul(X.d, A1.d, s.p1)
	p1 := craDigits{neg: X.neg != A1.neg && len(s.p1) > 0, d: s.p1}
	s.p2 = craMul(Y.d, A2.d, s.p2)
	p2 := craDigits{neg: Y.neg != A2.neg && len(s.p2) > 0, d: s.p2}
	v := craAddSub(p1, p2, s.sum)
	s.sum = v.d
	// rem: |v| mod bound with v's sign (Knuth division, mpn_manager::div).
	r := v.d
	if craCompare(v.d, B.d) >= 0 {
		na, nb := len(v.d), len(B.d)
		s.q = craGrow(s.q, na-nb+1)
		s.r = craGrow(s.r, nb)
		if _, e := m.Natural.divide(limbView{external: v.d, size: uint32(na)}, limbView{external: B.d, size: uint32(nb)},
			limbView{external: s.q, size: uint32(len(s.q))}, limbView{external: s.r, size: uint32(nb)}); e != nil {
			return true, e
		}
		r = craTrim(s.r)
	}
	neg := v.neg && len(r) > 0
	// mod: a negative remainder gains |bound|.
	if neg {
		t := craAddSub(craDigits{d: B.d}, craDigits{neg: true, d: r}, s.p1)
		s.p1 = t.d
		r, neg = t.d, false
	}
	// new_a > upper: new_a -= bound.
	if craCompare(r, U.d) > 0 {
		t := craAddSub(craDigits{d: r}, craDigits{neg: true, d: B.d}, s.p2)
		s.p2 = t.d
		r, neg = t.d, t.neg
	}
	if e := m.SetLimbs(out, r); e != nil {
		return true, e
	}
	if neg {
		if out.Cell == 0 {
			out.Val = -out.Val
		} else {
			out.Val = -1
		}
	}
	return true, nil
}

// DirectMulAdd computes mpzzp addmul(a, b, c, d) (submul when subtract; a
// nil a is zero) on digits: v = a + b*c (a - b*c), then p_normalize (in Zp
// mode the unique value of v's residue class in [Lower, Upper]), and
// publishes it into d as set does. It is DirectAddMul without the source
// temporaries, for a caller's coefficient loop under DirectKernel's
// conditions; it reports false, changing nothing, for an operand it does
// not read directly.
func (m *IntegerModularManager) DirectMulAdd(a, b, c, d *Int, subtract bool, s *CRAScratch) (bool, error) {
	if done, e := m.directWordMulAdd(a, b, c, d, subtract); done || e != nil {
		return done, e
	}
	z := m.manager
	var A craDigits
	ok := true
	if a != nil {
		A, ok = z.craOperand(a, &s.one[0])
	}
	B, ok2 := z.craOperand(b, &s.one[1])
	C, ok3 := z.craOperand(c, &s.one[2])
	if !(ok && ok2 && ok3) {
		return false, nil
	}
	var P, U craDigits
	if !m.z {
		var ok4, ok5 bool
		P, ok4 = z.craOperand(&m.P, &s.one[3])
		U, ok5 = z.craOperand(&m.Upper, &s.one[4])
		if !ok4 || !ok5 || P.neg || U.neg || len(P.d) == 0 {
			return false, nil
		}
	}
	s.p1 = craMul(B.d, C.d, s.p1)
	prod := craDigits{neg: (B.neg != C.neg) != subtract && len(s.p1) > 0, d: s.p1}
	v := craAddSub(A, prod, s.sum)
	s.sum = v.d
	r, neg := v.d, v.neg
	if !m.z {
		if craCompare(r, P.d) >= 0 {
			na, nb := len(r), len(P.d)
			s.q = craGrow(s.q, na-nb+1)
			s.r = craGrow(s.r, nb)
			if _, e := z.Natural.divide(limbView{external: r, size: uint32(na)}, limbView{external: P.d, size: uint32(nb)},
				limbView{external: s.q, size: uint32(len(s.q))}, limbView{external: s.r, size: uint32(nb)}); e != nil {
				return true, e
			}
			r = craTrim(s.r)
		}
		if neg && len(r) > 0 {
			t := craAddSub(craDigits{d: P.d}, craDigits{neg: true, d: r}, s.p2)
			s.p2 = t.d
			r = t.d
		}
		neg = false
		if craCompare(r, U.d) > 0 {
			t := craAddSub(craDigits{d: r}, craDigits{neg: true, d: P.d}, s.p1)
			s.p1 = t.d
			r, neg = t.d, t.neg
		}
	}
	if e := z.SetLimbs(d, r); e != nil {
		return true, e
	}
	if neg {
		if d.Cell == 0 {
			d.Val = -d.Val
		} else {
			d.Val = -1
		}
	}
	return true, nil
}
