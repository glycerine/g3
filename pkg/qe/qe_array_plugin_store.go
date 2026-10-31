package qe

// Source: src/qe/qe_array_plugin.cpp, solve_store.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// TryStoreEquality solves a store chain equated to rhs. It publishes the
// consistency constraints in the formula and appends one fresh value per store.
func (p *ArrayPlugin) TryStoreEquality(lhs, rhs, formula ast.ASTHandle) (solved bool, err error) {
	idx, levels, ok, e := p.arrayChain(lhs, rhs, ast.ASTArrayOpStore)
	if e != nil || !ok {
		return false, e
	}
	variable, e := p.context.Variable(idx)
	if e != nil {
		return false, e
	}
	result, e := p.manager.NewRef(formula)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&result)) }()
	replacement, e := p.manager.NewRef(rhs)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&replacement)) }()
	actual, e := p.manager.NewRef(rhs)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&actual)) }()
	step := func(level []ast.ASTHandle) (stepErr error) {
		if len(level) < 2 {
			return ast.ErrASTDomain
		}
		value := level[len(level)-1]
		sort, e := p.manager.GetSort(value)
		if e != nil {
			return e
		}
		fresh, e := p.manager.MkFreshConst("w", sort)
		if e != nil {
			return e
		}
		freshRef, e := p.manager.NewRef(fresh)
		if e != nil {
			return e
		}
		defer func() {
			if freshRef != (ast.ASTRef{}) {
				stepErr = errors.Join(stepErr, p.manager.DelRef(&freshRef))
			}
		}()
		base, e := p.manager.RefGet(actual)
		if e != nil {
			return e
		}
		selectArgs := append([]ast.ASTHandle{base}, level[:len(level)-1]...)
		selected, e := p.manager.MkFamilyApp(p.family, ast.ASTArrayOpSelect, nil, selectArgs)
		if e != nil {
			return e
		}
		selectedRef, e := p.manager.NewRef(selected)
		if e != nil {
			return e
		}
		defer func() { stepErr = errors.Join(stepErr, p.manager.DelRef(&selectedRef)) }()
		eq, e := p.manager.MkEq(selected, value)
		if e != nil {
			return e
		}
		eqRef, e := p.manager.NewRef(eq)
		if e != nil {
			return e
		}
		defer func() { stepErr = errors.Join(stepErr, p.manager.DelRef(&eqRef)) }()
		oldFormula, e := p.manager.RefGet(result)
		if e != nil {
			return e
		}
		newFormula, e := p.manager.MkAnd(oldFormula, eq)
		if e != nil {
			return e
		}
		if e = p.manager.SetRef(result, newFormula); e != nil {
			return e
		}
		realArgs := append([]ast.ASTHandle{base}, level...)
		realStore, e := p.manager.MkFamilyApp(p.family, ast.ASTArrayOpStore, nil, realArgs)
		if e != nil {
			return e
		}
		if e = p.manager.SetRef(actual, realStore); e != nil {
			return e
		}
		oldReplacement, e := p.manager.RefGet(replacement)
		if e != nil {
			return e
		}
		newArgs := append([]ast.ASTHandle{oldReplacement}, level[:len(level)-1]...)
		newArgs = append(newArgs, fresh)
		newStore, e := p.manager.MkFamilyApp(p.family, ast.ASTArrayOpStore, nil, newArgs)
		if e != nil {
			return e
		}
		if e = p.manager.SetRef(replacement, newStore); e != nil {
			return e
		}
		if e = p.context.adoptVariable(fresh, &freshRef); e != nil {
			return e
		}
		return nil
	}
	for i := len(levels) - 1; i >= 0; i-- {
		if e = step(levels[i]); e != nil {
			return false, e
		}
	}
	replacementExpr, e := p.manager.RefGet(replacement)
	if e != nil {
		return false, e
	}
	rw, e := rewriter.NewExprSafeReplace(p.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	if e = rw.ApplySubstitution(variable, replacementExpr, result); e != nil {
		return false, e
	}
	rewritten, e := p.manager.RefGet(result)
	if e != nil {
		return false, e
	}
	if e = p.context.EliminateVariable(idx, rewritten, replacementExpr); e != nil {
		return false, e
	}
	return true, nil
}
