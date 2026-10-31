package qe

// Source: src/qe/qe.cpp, same-polarity variable block elimination.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// eliminateQuantifierMixed separates Boolean and arithmetic declarations of
// a single quantifier. Every group must be fully eliminated by its concrete
// solver; unsupported declarations or group results report incomplete.
func eliminateQuantifierMixed(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		return result, false, err
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, false, err
	}
	if err = free.Collect(quantifier); err != nil || !free.Empty() {
		return result, false, err
	}
	body, err := m.GetQuantifierBody(quantifier)
	if err != nil {
		return result, false, err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return result, false, err
	}
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return result, false, err
	}
	var boolVars, arithmeticVars []ast.ASTHandle
	for _, ref := range extracted.variables {
		variable, e := m.RefGet(ref)
		if e != nil {
			return result, false, e
		}
		sort, e := m.GetSort(variable)
		if e != nil {
			return result, false, e
		}
		if sort == boolean {
			boolVars = append(boolVars, variable)
			continue
		}
		family, e := m.GetFamilyID(sort)
		if e != nil {
			return result, false, e
		}
		if family != arithmetic {
			return result, false, nil
		}
		arithmeticVars = append(arithmeticVars, variable)
	}
	if len(boolVars) == 0 || len(arithmeticVars) == 0 {
		return result, false, nil
	}
	body, err = m.RefGet(extracted.body)
	if err != nil {
		return result, false, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	formula, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	for i, group := range [][]ast.ASTHandle{boolVars, arithmeticVars} {
		body, e := m.RefGet(formula)
		if e != nil {
			return result, false, e
		}
		bound, e := bindExprQuantifierVariables(m, group, body)
		if e != nil {
			return result, false, e
		}
		q, e := m.RefGet(bound)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		kind, e := m.GetKind(q)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		if kind != ast.ASTQuantifier {
			if e = m.DelRef(&bound); e != nil {
				return result, false, e
			}
			continue
		}
		var eliminated ast.ASTRef
		var done bool
		if i == 0 {
			eliminated, done, e = eliminateQuantifierBoolUnits(m, q, config)
		} else {
			eliminated, done, e = eliminateQuantifierArithmeticEquations(m, q, config)
			if e == nil && !done {
				eliminated, done, e = eliminateQuantifierArithmeticClosed(m, q, config)
			}
			if e == nil && !done {
				eliminated, done, e = eliminateQuantifierArithmeticSearch(m, q, config)
			}
		}
		if e != nil || !done {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		value, e := m.RefGet(eliminated)
		if e == nil {
			e = m.SetRef(formula, value)
		}
		e = errors.Join(e, m.DelRef(&eliminated), m.DelRef(&bound))
		if e != nil {
			return result, false, e
		}
	}
	body, err = m.RefGet(formula)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNotSimplified(body)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(body)
	return result, err == nil, err
}
