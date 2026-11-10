package rewriter

// Source: src/ast/rewriter/poly_rewriter.h mk_add(a1, a2, result) and
// mk_sub(a1, a2, result); poly_rewriter_def.h mk_add_app.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// MkAdd2 is poly_rewriter::mk_add(a1, a2, result): mk_add_core, or the
// plain application when it fails. The result is not rewritten further.
// The caller owns the returned reference.
func (r *ArithRewriter) MkAdd2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkBinary(arithOpAdd, a1, a2)
}

// MkSub2 is poly_rewriter::mk_sub(a1, a2, result): a1 + (* -1 a2), which
// is never BR_FAILED.
func (r *ArithRewriter) MkSub2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkBinary(arithOpSub, a1, a2)
}

func (r *ArithRewriter) mkBinary(kind int32, a1, a2 ast.ASTHandle) (out ast.ASTRef, err error) {
	if err = r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	m := r.manager
	sort, err := m.GetSort(a1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	decl, err := m.MkFamilyFuncDecl(r.family, kind, nil, []ast.ASTHandle{sort, sort}, sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return ast.ASTRef{}, err
	}
	args := []ast.ASTHandle{a1, a2}
	status, err := r.MkAppCore(decl, ast.SliceArguments(args), out)
	if err == nil && status == RewriteFailed {
		// mk_add_app(2, args)
		var app ast.ASTHandle
		if app, err = m.MkFamilyApp(r.family, kind, nil, args); err == nil {
			err = m.SetRef(out, app)
		}
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&out))
	}
	return out, nil
}
