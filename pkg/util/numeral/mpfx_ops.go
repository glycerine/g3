package numeral

// Source: src/util/mpfx.cpp:558-650 and src/util/mpfx.h:260-278.

// Floor is the source floor: it rounds toward minus infinity in place.
func (m *FixedPointManager) Floor(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	m.traceFixedRoundIntegral(a, false, false)
	if err := m.roundIntegral(a, a.isNeg()); err != nil {
		return err
	}
	m.traceFixedRoundIntegral(a, false, true)
	return nil
}

// Ceil is the source ceil: it rounds toward plus infinity in place.
func (m *FixedPointManager) Ceil(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	m.traceFixedRoundIntegral(a, true, false)
	if err := m.roundIntegral(a, a.isPos()); err != nil {
		return err
	}
	m.traceFixedRoundIntegral(a, true, true)
	return nil
}

// roundIntegral is the shared floor/ceil body: the fractional words are
// cleared, and a nonzero fraction increments the integer words when away is
// set. The source writes the zero slot's shared words; g3's zero slot has
// none, and it stays the zero slot.
func (m *FixedPointManager) roundIntegral(n *FixedPoint, away bool) error {
	if n.isZero() {
		m.reset(n)
		return nil
	}
	w := n.words
	frac := m.fracWords
	isInt := fixedWordsZero(w[:frac])
	clear(w[:frac])
	if away && !isInt && !fixedWordsInc(w[frac:]) {
		return ErrFixedPointOverflow
	}
	if fixedWordsZero(w[frac:]) {
		m.reset(n)
	}
	return nil
}

// Inv is the source inv: div(m_one, a, a) under the current rounding.
func (m *FixedPointManager) Inv(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	if a.isZero() || fixedWordsZero(a.words) {
		return ErrFixedPointDivZero
	}
	m.traceFixedInverse(a, false)
	if err := m.div(&m.one, a, a); err != nil {
		return err
	}
	m.traceFixedInverse(a, true)
	return nil
}

// Power preserves the source's left-to-right small-power path for distinct
// operands and repeated-square path for aliased or larger powers. Directed
// rounding occurs after each multiplication, not only at the final result.
func (m *FixedPointManager) Power(a *FixedPoint, exponent uint32, out *FixedPoint) (err error) {
	if err := m.pair(a, out); err != nil {
		return err
	}
	input := m.fixedPowerTraceInput(a)
	defer func() {
		if err == nil {
			m.traceFixedPower(input, exponent, out)
		}
	}()
	if a.isZero() {
		// The source asserts p != 0 (debug only); the release build resets
		// b, so 0^0 is 0.
		return m.Reset(out)
	}
	switch exponent {
	case 0:
		return m.SetInt64(out, 1)
	case 1:
		return m.Copy(out, a)
	case 2:
		return m.Mul(a, a, out)
	}
	if exponent <= 8 && a != out {
		if err := m.Copy(out, a); err != nil {
			return err
		}
		for i := uint32(1); i < exponent; i++ {
			if err := m.Mul(a, out, out); err != nil {
				return err
			}
		}
		return nil
	}
	// The source's 32-bit mask loop would wrap to zero for exponents with
	// the high bit set. Refuse that nonterminating input explicitly.
	if exponent >= 1<<31 {
		return ErrFixedPointDomain
	}
	pw, err := m.New()
	if err != nil {
		return err
	}
	defer pw.Close()
	if err := m.Copy(pw, a); err != nil {
		return err
	}
	if err := m.SetInt64(out, 1); err != nil {
		return err
	}
	for mask := uint32(1); mask <= exponent; mask <<= 1 {
		if mask&exponent != 0 {
			if err := m.Mul(out, pw, out); err != nil {
				return err
			}
		}
		// The source squares even after the highest selected bit.
		if err := m.Mul(pw, pw, pw); err != nil {
			return err
		}
	}
	return nil
}
