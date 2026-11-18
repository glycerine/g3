package tactic

// Source: src/math/subpaving/tactic/expr2subpaving.cpp:173-178, 260-266.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// A leading multiplication coefficient is read directly by process_mul.
// It is not a recursive process visit and must not charge the AST resource
// limit or cooperate as a separate expression.
func copyExprCoefficient(qm *numeral.RationalManager, text string, n, d *numeral.Int) (err error) {
	coefficient := numeral.NewRational()
	defer func() { err = errors.Join(err, qm.Del(&coefficient)) }()
	if err = qm.SetString(&coefficient, text); err != nil {
		return err
	}
	if err = qm.Numerator(&coefficient, n); err != nil {
		return err
	}
	return qm.Denominator(&coefficient, d)
}

// astUnsignedNumeral is arith_util::is_numeral(h, k) && k.is_unsigned(),
// returning k.get_unsigned(). k is a rational of the AST's manager.
func astUnsignedNumeral(m *ast.ASTManager, h ast.ASTHandle) (value uint32, ok bool, err error) {
	q := m.NumeralService().Manager
	k := numeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&k)) }()
	_, matched, err := m.ArithmeticNumeralRational(h, &k)
	if err != nil || !matched {
		return 0, false, err
	}
	if ok, err = q.IsUnsigned(&k); err != nil || !ok {
		return 0, false, err
	}
	value, err = q.Unsigned(&k)
	return value, err == nil, err
}
