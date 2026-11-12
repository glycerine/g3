package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) MkAtom(expression ast.ASTHandle, positive bool) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	r, handled, err := p.util.NormalizeAtom(expression, positive)
	if err != nil || !handled {
		return ast.ASTHandle{}, handled, err
	}
	h, err := p.manager.RefGet(r)
	if err == nil {
		p.normalizedAtoms = append(p.normalizedAtoms, r)
	}
	if err != nil {
		_ = p.manager.DelRef(&r)
	}
	return h, err == nil, err
}

// mkAtomRef is mk_atom returning its expr_ref result to the caller.
func (p *ArithmeticPlugin) mkAtomRef(expression ast.ASTHandle, positive bool) (ast.ASTRef, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTRef{}, false, err
	}
	return p.util.NormalizeAtom(expression, positive)
}

// IsUninterpreted retains the source classification. A non-numeral mod and a
// product with no numeral factor are treated as opaque.
func (p *ArithmeticPlugin) IsUninterpreted(application ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if k, err := p.manager.GetKind(application); err != nil || k != ast.ASTApp {
		return false, errors.Join(ast.ErrASTDomain, err)
	}
	family, err := p.manager.GetFamilyID(application)
	if err != nil {
		return false, err
	}
	if family != p.family {
		return true, nil
	}
	kind, err := p.manager.GetDeclKind(application)
	if err != nil {
		return false, err
	}
	switch kind {
	case 0, 2, 3, 4, 5, 6, 7, 8:
		return false, nil
	case 13:
		d, err := p.manager.GetArg(application, 1)
		if err != nil {
			return false, err
		}
		_, numeral, err := p.util.numeral(d)
		return !numeral, err
	case 9:
		a, err := p.manager.GetArg(application, 0)
		if err != nil {
			return false, err
		}
		b, err := p.manager.GetArg(application, 1)
		if err != nil {
			return false, err
		}
		_, an, err := p.util.numeral(a)
		if err != nil {
			return false, err
		}
		_, bn, err := p.util.numeral(b)
		return !(an || bn), err
	default:
		return true, nil
	}
}

func (p *ArithmeticPlugin) Weight(*ast.ContainsApp, ast.ASTHandle) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return 2, nil
}

// Solve is arith_plugin::solve: m_util.solve(conjs, fml).
func (p *ArithmeticPlugin) Solve(conjunctions *ConjunctionEnum, formula ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.solve(conjunctions, formula)
}
