package qe

// Source: src/qe/qe_array_plugin.cpp, solve_eq's direct-variable branch.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// TryDirectEquality eliminates a context variable equated to an expression
// that does not contain it. Store and select equation solving are separate
// source branches and are not attempted here.
func (p *ArrayPlugin) TryDirectEquality(eq, formula ast.ASTHandle) (solved bool, err error) {
	if err = p.open(); err != nil {
		return false, err
	}
	kind, err := p.manager.GetKind(eq)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := p.manager.GetFamilyID(eq)
	if err != nil {
		return false, err
	}
	op, err := p.manager.GetDeclKind(eq)
	if err != nil {
		return false, err
	}
	if family != p.manager.BasicFamilyID() || op != ast.ASTOpEq {
		return false, nil
	}
	left, err := p.manager.GetArg(eq, 0)
	if err != nil {
		return false, err
	}
	right, err := p.manager.GetArg(eq, 1)
	if err != nil {
		return false, err
	}
	for side := 0; side < 2; side++ {
		k, e := p.manager.GetKind(left)
		if e != nil {
			return false, e
		}
		if k == ast.ASTApp {
			idx, found, e := p.context.IsVariable(left)
			if e != nil {
				return false, e
			}
			if found {
				contains, e := p.context.Contains(idx)
				if e != nil {
					return false, e
				}
				has, e := contains.Check(right)
				if e != nil {
					return false, e
				}
				if !has {
					result, e := p.manager.NewRef(formula)
					if e != nil {
						return false, e
					}
					defer func() { err = errors.Join(err, p.manager.DelRef(&result)) }()
					rw, e := rewriter.NewExprSafeReplace(p.manager)
					if e != nil {
						return false, e
					}
					defer func() { err = errors.Join(err, rw.Close()) }()
					if e = rw.ApplySubstitution(left, right, result); e != nil {
						return false, e
					}
					rewritten, e := p.manager.RefGet(result)
					if e != nil {
						return false, e
					}
					if e = p.context.EliminateVariable(idx, rewritten, right); e != nil {
						return false, e
					}
					return true, nil
				}
			}
		}
		left, right = right, left
	}
	return false, nil
}
