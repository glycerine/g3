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

// MkLe2 is arith_rewriter::mk_le(arg1, arg2, result): mk_le_core, or the
// plain application when it fails. The caller owns the returned reference.
func (r *ArithRewriter) MkLe2(a1, a2 ast.ASTHandle) (out ast.ASTRef, err error) {
	if err = r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	m := r.manager
	sort, err := m.GetSort(a1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTRef{}, err
	}
	decl, err := m.MkFamilyFuncDecl(r.family, arithOpLE, nil, []ast.ASTHandle{sort, sort}, boolSort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return ast.ASTRef{}, err
	}
	args := []ast.ASTHandle{a1, a2}
	status, err := r.MkAppCore(decl, ast.SliceArguments(args), out)
	if err == nil && status == RewriteFailed {
		var app ast.ASTHandle
		if app, err = m.MkFamilyApp(r.family, arithOpLE, nil, args); err == nil {
			err = m.SetRef(out, app)
		}
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&out))
	}
	return out, nil
}

// MkMod2 is arith_rewriter::mk_mod(arg1, arg2, result): mk_mod_core, or
// the plain application when it fails. The caller owns the returned
// reference.
func (r *ArithRewriter) MkMod2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkBinary(arithOpMod, a1, a2)
}

// MkAddN is poly_rewriter::mk_add(num_args, args, result): mk_add_core, or
// mk_add_app (0 -> numeral 0, 1 -> the argument, else the plain
// application) when it fails. The caller owns the returned reference.
func (r *ArithRewriter) MkAddN(args []ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkConvenience(arithOpAdd, args)
}

// MkMul2 is poly_rewriter::mk_mul(a1, a2, result): mk_mul_core, or
// mk_mul_app(2, args) when it fails.
func (r *ArithRewriter) MkMul2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkConvenience(arithOpMul, []ast.ASTHandle{a1, a2})
}

// MkUMinus1 is poly_rewriter::mk_uminus(arg, result), which is never
// BR_FAILED.
func (r *ArithRewriter) MkUMinus1(a ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkConvenience(arithOpUMinus, []ast.ASTHandle{a})
}

// MkLt2 is arith_rewriter::mk_lt(arg1, arg2, result) = mk_lt_core, which
// is never BR_FAILED.
func (r *ArithRewriter) MkLt2(a1, a2 ast.ASTHandle) (ast.ASTRef, error) {
	return r.mkConvenience(arithOpLT, []ast.ASTHandle{a1, a2})
}

// mkConvenience runs mk_app_core's case for kind once (no further
// rewriting of the result) and applies the source fallback on BR_FAILED.
func (r *ArithRewriter) mkConvenience(kind int32, args []ast.ASTHandle) (out ast.ASTRef, err error) {
	if err = r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	m := r.manager
	if len(args) == 0 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	sort, err := m.GetSort(args[0])
	if err != nil {
		return ast.ASTRef{}, err
	}
	rng := sort
	if kind == arithOpLT || kind == arithOpLE {
		if rng, err = m.MkBoolSort(); err != nil {
			return ast.ASTRef{}, err
		}
	}
	domain := make([]ast.ASTHandle, len(args))
	for i := range domain {
		domain[i] = sort
	}
	decl, err := m.MkFamilyFuncDecl(r.family, kind, nil, domain, rng)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return ast.ASTRef{}, err
	}
	status, err := r.MkAppCore(decl, ast.SliceArguments(args), out)
	if err == nil && status == RewriteFailed {
		var app ast.ASTHandle
		switch kind {
		case arithOpAdd:
			// mk_add_app(num_args, args)
			if len(args) == 1 {
				app = args[0]
			} else {
				app, err = m.MkFamilyApp(r.family, kind, nil, args)
			}
		case arithOpMul:
			app, err = r.mkMulApp(args, args[0])
		default:
			app, err = m.MkFamilyApp(r.family, kind, nil, args)
		}
		if err == nil {
			err = m.SetRef(out, app)
		}
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&out))
	}
	return out, nil
}
