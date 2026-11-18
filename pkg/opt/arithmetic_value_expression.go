package opt

// Source: src/opt/opt_context.cpp:1352-1381, context::to_expr, adapted to
// the Go simplex value used by the retained arithmetic theories.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// ArithmeticValueExpression converts c*infinity+a+b*epsilon (a value of the
// AST manager's rational manager) into an owned AST reference suitable for
// SMT-LIB objective display.
func ArithmeticValueExpression(m *ast.ASTManager, value *simplex.Infinitesimal, integer bool) (_ ast.ASTRef, result error) {
	if m == nil || value == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	var args []ast.ASTHandle
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&roots[i]))
		}
	}()
	hold := func(h ast.ASTHandle, err error) error {
		if err != nil {
			return err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		args = append(args, h)
		return nil
	}
	o := newRatOps(m)
	defer func() { result = o.joined(result) }()
	coefficientTerm := func(coefficient *numeral.Rational, name string, termInteger bool) (ast.ASTHandle, error) {
		family, err := m.ArithmeticFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sortKind := int32(0)
		if termInteger {
			sortKind = 1
		}
		sort, err := m.MkFamilySort(family, sortKind, nil)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		symbol, err := m.Symbols.Intern(name)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, err := m.MkConst(symbol, sort)
		if err != nil || o.isOne(coefficient) {
			return term, err
		}
		numeral, err := m.ArithmeticNumeralFromRational(coefficient, termInteger)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return m.MkArithmeticMul(numeral, term)
	}
	// bool is_int = eps.is_zero() && r.is_int(): from the value, not the
	// objective's sort.
	_ = integer
	integer = o.sign(&value.Epsilon) == 0 && o.isInt(&value.Main)
	if o.sign(&value.Infinity) != 0 {
		term, err := coefficientTerm(&value.Infinity, "oo", integer)
		if err = hold(term, err); err != nil {
			return ast.ASTRef{}, err
		}
	}
	if o.sign(&value.Main) != 0 {
		if err := hold(m.ArithmeticNumeralFromRational(&value.Main, integer)); err != nil {
			return ast.ASTRef{}, err
		}
	}
	if o.sign(&value.Epsilon) != 0 {
		term, err := coefficientTerm(&value.Epsilon, "epsilon", false)
		if err = hold(term, err); err != nil {
			return ast.ASTRef{}, err
		}
	}
	var expression ast.ASTHandle
	var err error
	switch len(args) {
	case 0:
		zero := numeral.NewRational()
		expression, err = m.ArithmeticNumeralFromRational(&zero, true)
	case 1:
		expression = args[0]
	default:
		var family ast.FamilyID
		if family, err = m.ArithmeticFamilyID(); err == nil {
			expression, err = m.MkFamilyApp(family, 6, nil, args) // m_arith.mk_add(n, args)
		}
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(expression)
}
