package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp is_number and mk_numeral.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// bvBoundsNumeral is m_bv.is_numeral(e, r, sz) with r.get_uint64() for a
// numeral of at most 64 bits (value is zero for a wider numeral).
func bvBoundsNumeral(m *ast.ASTManager, h ast.ASTHandle) (value uint64, width uint32, numeral bool, resultErr error) {
	q := m.NumeralService().Manager
	r := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&r)) }()
	width, numeral, err := m.BitVectorNumeralRational(h, &r)
	if err != nil || !numeral {
		return 0, 0, false, err
	}
	if width > 64 {
		return 0, width, true, nil
	}
	value, err = q.Uint64(&r)
	return value, width, err == nil, err
}

// bvBoundsMakeNumeral is m_bv.mk_numeral(rational(v, rational::ui64()), sz).
func bvBoundsMakeNumeral(m *ast.ASTManager, value uint64, width uint32) (ast.ASTHandle, error) {
	return m.BitVectorNumeralUint64(value, width)
}
