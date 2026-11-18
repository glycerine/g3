package numeral

// Source: src/util/mpfx.cpp:864-868 and src/util/mpfx.h:384.

// PrevPowerOfTwo returns the exponent of the highest set bit in the integer
// part. The source unsigned subtraction wraps for a positive value below one;
// the retained nth-root caller only invokes this method for values at least one.
func (m *FixedPointManager) PrevPowerOfTwo(a *FixedPoint) (uint32, error) {
	if err := m.check(a); err != nil {
		return 0, err
	}
	if !a.isPos() {
		return 0, nil
	}
	return m.intWords*32 - fixedWordsNLZ(a.words[m.fracWords:]) - 1, nil
}
