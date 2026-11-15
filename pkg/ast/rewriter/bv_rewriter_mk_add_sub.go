package rewriter

// Source: src/ast/rewriter/poly_rewriter.h mk_add(a1, a2, result) and
// mk_sub(a1, a2, result), on bv_rewriter.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// MkAdd2 is poly_rewriter::mk_add(a1, a2, result) on a bv_rewriter: one
// mk_add_core step, or the plain bvadd when it fails. The result is not
// rewritten further. The caller owns the returned reference.
func (p *BVRewriter) MkAdd2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return p.mkPair(a1, a2, true)
}

// MkSub2 is poly_rewriter::mk_sub(a1, a2, result): the raw
// (bvadd a1 (bvmul -1 a2)); its BR_REWRITE2 status is ignored.
func (p *BVRewriter) MkSub2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return p.mkPair(a1, a2, false)
}

func (p *BVRewriter) mkPair(a1, a2 ast.ASTHandle, add bool) (out ast.ASTRef, err error) {
	if err = p.open(); err != nil {
		return ast.ASTRef{}, err
	}
	m := p.manager
	if out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return ast.ASTRef{}, err
	}
	if add {
		err = p.polynomial.MakeAdditionPair(a1, a2, out)
	} else {
		err = p.polynomial.MakeSubtractionPair(a1, a2, out)
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&out))
	}
	return out, nil
}
