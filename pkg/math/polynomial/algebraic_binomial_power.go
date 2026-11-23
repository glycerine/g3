package polynomial

// BinomialPowerFraction recognizes a defining polynomial a*X^k + b whose
// two nonzero coefficients fit in signed words. Every one of its roots has
// the exact kth power -b/a, independently of its isolating interval.
func (m *AlgebraicRootCore) BinomialPowerFraction(value *Algebraic, k uint32) (numerator, denominator int64, ok bool, err error) {
	loc, err := m.location(value)
	if err != nil || loc == nil || loc.kind == 0 || k < 2 {
		return 0, 0, false, err
	}
	cell, err := m.load(loc)
	if err != nil || cell.size == 0 || cell.size-1 != k {
		return 0, 0, false, err
	}
	p, err := m.sourcePolynomial(&cell)
	if err != nil {
		return 0, 0, false, err
	}
	pm := m.config.Polynomials
	leading, err := pm.read(p, k)
	if err != nil || !leading.IsSmall() || leading.Val == 0 {
		return 0, 0, false, err
	}
	constant, err := pm.read(p, 0)
	if err != nil || !constant.IsSmall() || constant.Val == 0 {
		return 0, 0, false, err
	}
	for j := uint32(1); j < k; j++ {
		c, err := pm.read(p, j)
		if err != nil || !c.IsSmall() || c.Val != 0 {
			return 0, 0, false, err
		}
	}
	return -int64(constant.Val), int64(leading.Val), true, nil
}
