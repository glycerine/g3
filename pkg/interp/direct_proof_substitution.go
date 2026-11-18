package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp subst_term_and_simp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// SubstituteProofTerm substitutes one symbolic proof variable while retaining
// the source barriers for rotation pivots and concatenated rewrite chains.
func (p *DirectProofInterpolator) SubstituteProofTerm(variable, replacement, expression ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	// subst_term_and_simp clears the member subst_memo; its entries hold
	// their terms until the next substitution or the destructor.
	if err := p.substMemoRefs.clear(p.base.manager.ast); err != nil {
		return ast.ASTHandle{}, err
	}
	var substitute func(ast.ASTHandle) (ast.ASTHandle, error)
	substitute = func(current ast.ASTHandle) (ast.ASTHandle, error) {
		if current == variable {
			return replacement, nil
		}
		if result, found := memo[current]; found {
			return result, nil
		}
		entry, err := p.substMemoRefs.lookup(p.base.manager.ast, current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		record := func(result ast.ASTHandle) error {
			memo[current] = result
			return p.substMemoRefs.setValue(p.base.manager.ast, entry, 0, result)
		}
		op, err := p.base.manager.Operator(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if op == OperatorUninterpreted {
			declaration, e := p.base.manager.Symbol(current)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if declaration == p.rotateSum {
				pivot, e := p.base.manager.Arg(current, 0)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				pivotPlaceholder, e := p.placeholder(pivot)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if variable == pivotPlaceholder {
					return current, record(current)
				}
				proof, e := p.base.manager.Arg(current, 1)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				proof, e = substitute(proof)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				result, e := p.makeSymbol(p.rotateSum, pivot, proof)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				return result, record(result)
			}
			if declaration == p.concat {
				return current, record(current)
			}
		}
		args, err := p.base.manager.Args(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for i := range args {
			args[i], err = substitute(args[i])
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		var result ast.ASTHandle
		switch {
		case op == OperatorEqual && len(args) == 2 && args[0] == args[1]:
			result, err = p.base.manager.Make(OperatorTrue)
		case op == OperatorAnd:
			result, err = p.base.manager.MkAndSlice(args)
		case op == OperatorOr:
			result, err = p.base.manager.MkOrSlice(args)
		case op == OperatorIDiv && len(args) == 2:
			var ref ast.ASTRef
			ref, err = p.base.manager.MakeIDiv(args[0], args[1])
			if err == nil {
				result, err = p.base.manager.ast.RefGet(ref)
				if err == nil {
					p.owned = append(p.owned, ref)
				} else {
					err = errors.Join(err, p.base.manager.ast.DelRef(&ref))
				}
			}
		default:
			result, err = p.base.manager.Clone(current, args)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return result, record(result)
	}
	result, err := substitute(expression)
	return p.keep(result, err)
}
