package interp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// interpSmall is the small rational n/d (lowest terms, d > 0) without
// numeral storage.
func interpSmall(n, d int32) *numeral.Rational {
	return &numeral.Rational{Num: numeral.NewInt(n), Den: numeral.NewInt(d)}
}

// interpNumeral is Manager.Numeral as a test oracle *big.Rat.
func interpNumeral(m *Manager, h ast.ASTHandle) (*big.Rat, bool, error) {
	var out *big.Rat
	var matched bool
	err := m.withRational(func(r *numeral.Rational) error {
		var e error
		if matched, e = m.Numeral(h, r); e != nil || !matched {
			return e
		}
		out = new(big.Rat)
		return m.Rationals().BigRatOf(r, out)
	})
	return out, matched, err
}

// interpCoefficient is Manager.Coefficient as a test oracle *big.Rat.
func interpCoefficient(m *Manager, h ast.ASTHandle) (*big.Rat, error) {
	out := new(big.Rat)
	err := m.withRational(func(r *numeral.Rational) error {
		if e := m.Coefficient(h, r); e != nil {
			return e
		}
		return m.Rationals().BigRatOf(r, out)
	})
	return out, err
}

// interpRats converts and releases owned rationals.
func interpRats(t *testing.T, m *Manager) func([]numeral.Rational, error) ([]*big.Rat, error) {
	return func(values []numeral.Rational, err error) ([]*big.Rat, error) {
		return interpRatsOf(t, m, values, err)
	}
}

func interpRatsOf(t *testing.T, m *Manager, values []numeral.Rational, err error) ([]*big.Rat, error) {
	t.Helper()
	if err != nil {
		return nil, err
	}
	out := make([]*big.Rat, len(values))
	for i := range values {
		out[i] = new(big.Rat)
		if e := m.Rationals().BigRatOf(&values[i], out[i]); e != nil {
			t.Fatal(e)
		}
	}
	return out, m.releaseRationals(values)
}
