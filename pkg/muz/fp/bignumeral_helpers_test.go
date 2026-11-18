package fp

// Test helpers: the tests state numerals as math/big oracle values; these
// build and read AST numerals through the rational (numeral) API.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// bigTestRational converts v into a rational of m's rational manager; the
// caller frees it.
func bigTestRational(m *ast.ASTManager, v *big.Rat) (numeral.Rational, error) {
	r := numeral.NewRational()
	if v == nil {
		return r, ast.ErrASTDomain
	}
	q := m.NumeralService().Manager
	if err := q.SetString(&r, v.RatString()); err != nil {
		return r, errors.Join(err, q.Del(&r))
	}
	return r, nil
}

// bigTestArithNumeral is arith_util::mk_numeral(v, is_int).
func bigTestArithNumeral(m *ast.ASTManager, v *big.Rat, integer bool) (_ ast.ASTHandle, resultErr error) {
	r, err := bigTestRational(m, v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.NumeralService().Manager.Del(&r)) }()
	return m.ArithmeticNumeralFromRational(&r, integer)
}

// bigTestBVNumeral is bv_util::mk_numeral(v, bits).
func bigTestBVNumeral(m *ast.ASTManager, v *big.Rat, bits uint32) (_ ast.ASTHandle, resultErr error) {
	r, err := bigTestRational(m, v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.NumeralService().Manager.Del(&r)) }()
	return m.BitVectorNumeralFromRational(&r, bits)
}

// bigTestParameter is parameter(rational v).
func bigTestParameter(m *ast.ASTManager, v *big.Rat) (_ ast.ASTParameter, resultErr error) {
	r, err := bigTestRational(m, v)
	if err != nil {
		return ast.ASTParameter{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.NumeralService().Manager.Del(&r)) }()
	return m.NewParameterFromRational(&r)
}

// bigTestBig reads a rational of m's rational manager as a big.Rat.
func bigTestBig(m *ast.ASTManager, r *numeral.Rational) (*big.Rat, error) {
	text, err := m.NumeralService().Manager.String(r)
	if err != nil {
		return nil, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return value, nil
}

// bigTestArithValue is arith_util::is_numeral(h, v, is_int) read as a
// big.Rat.
func bigTestArithValue(m *ast.ASTManager, h ast.ASTHandle) (_ *big.Rat, integer, matched bool, resultErr error) {
	r := numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, m.NumeralService().Manager.Del(&r)) }()
	integer, matched, err := m.ArithmeticNumeralRational(h, &r)
	if err != nil || !matched {
		return nil, integer, matched, err
	}
	value, err := bigTestBig(m, &r)
	return value, integer, matched, err
}

// bigTestBVValue is bv_util::is_numeral(h, v, bv_size) read as a big.Int.
func bigTestBVValue(m *ast.ASTManager, h ast.ASTHandle) (_ *big.Int, width uint32, matched bool, resultErr error) {
	r := numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, m.NumeralService().Manager.Del(&r)) }()
	width, matched, err := m.BitVectorNumeralRational(h, &r)
	if err != nil || !matched {
		return nil, width, matched, err
	}
	value, err := bigTestBig(m, &r)
	if err != nil {
		return nil, width, matched, err
	}
	return value.Num(), width, matched, nil
}
