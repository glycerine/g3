package numeral

// Source: src/util/mpfx.h:160-247 and src/util/mpfx.cpp:316-417.

func (m *FixedPointManager) IsPos(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.isPos(), nil
}

func (m *FixedPointManager) IsNeg(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.isNeg(), nil
}

func (m *FixedPointManager) IsNonPos(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return !a.isPos(), nil
}

func (m *FixedPointManager) IsNonNeg(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return !a.isNeg(), nil
}

func (m *FixedPointManager) IsOne(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.isPos() && m.isAbsOne(a), nil
}

func (m *FixedPointManager) IsMinusOne(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.isNeg() && m.isAbsOne(a), nil
}

func (m *FixedPointManager) Equal(a, b *FixedPoint) (bool, error) {
	if err := m.pair(a, b); err != nil {
		return false, err
	}
	return m.eq(a, b), nil
}

// isAbsOne is the source is_abs_one.
func (m *FixedPointManager) isAbsOne(n *FixedPoint) bool {
	w := m.view(n)
	return m.isInt(n) && w[m.fracWords] == 1 && fixedWordsZero(w[m.fracWords+1:])
}

func (m *FixedPointManager) Less(a, b *FixedPoint) (bool, error) {
	if err := m.pair(a, b); err != nil {
		return false, err
	}
	m.traceFixedLess(a, b, false, false)
	less := m.lt(a, b)
	m.traceFixedLess(a, b, true, less)
	return less, nil
}

func (m *FixedPointManager) LessEqual(a, b *FixedPoint) (bool, error) {
	less, err := m.Less(b, a)
	if err != nil {
		return false, err
	}
	return !less, nil
}

func (m *FixedPointManager) Greater(a, b *FixedPoint) (bool, error) {
	return m.Less(b, a)
}

func (m *FixedPointManager) GreaterEqual(a, b *FixedPoint) (bool, error) {
	less, err := m.Less(a, b)
	if err != nil {
		return false, err
	}
	return !less, nil
}

// Inc and Dec are the source add/sub with the manager's m_one.
func (m *FixedPointManager) Inc(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	return m.add(a, &m.one, a)
}

func (m *FixedPointManager) Dec(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	return m.sub(a, &m.one, a)
}

// Swap exchanges owned values without exchanging their manager bindings.
func (m *FixedPointManager) Swap(a, b *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if a == b {
		return nil
	}
	a.sign, b.sign = b.sign, a.sign
	a.words, b.words = b.words, a.words
	return nil
}
