package numeral

import "math/big"

// Rational returns an independent exact math/big value of a: a test bridge
// that lets tests check the word arithmetic against math/big. No production
// code calls it.
func (m *FixedPointManager) Rational(a *FixedPoint) (*big.Rat, error) {
	if err := m.check(a); err != nil {
		return nil, err
	}
	num := new(big.Int)
	ws := m.view(a)
	for i := len(ws); i > 0; {
		i--
		num.Lsh(num, 32)
		num.Or(num, new(big.Int).SetUint64(uint64(ws[i])))
	}
	if a.isNeg() {
		num.Neg(num)
	}
	den := new(big.Int).Lsh(big.NewInt(1), uint(32*m.fracWords))
	return new(big.Rat).SetFrac(num, den), nil
}
