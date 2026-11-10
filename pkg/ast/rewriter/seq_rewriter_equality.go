package rewriter

// Source: src/ast/rewriter/seq_rewriter.cpp:1572-1588 (mk_eq_core).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RewriteEquality is seq_rewriter::mk_eq_core.
func (r *SequenceRewriter) RewriteEquality(left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	m := r.manager
	// expr_ref_vector lhs(m()), rhs(m()), res(m()); destroyed res, rhs, lhs.
	var lhsV, rhsV, resV seqMemberVector
	defer func() {
		err = errors.Join(err, resV.reset(m), rhsV.reset(m), lhsV.reset(m))
	}()
	var lhs, rhs []ast.ASTHandle
	changed := false
	ok, err := r.ReduceEqExpr(left, right, &lhs, &rhs, &changed)
	// The appended terms belong to the caller's vectors.
	if e := lhsV.adopt(m, lhs); e != nil {
		return RewriteFailed, errors.Join(err, e)
	}
	if e := rhsV.adopt(m, rhs); e != nil {
		return RewriteFailed, errors.Join(err, e)
	}
	if err != nil {
		return RewriteFailed, err
	}
	if !ok {
		return r.assignBoolean(false, out)
	}
	if !changed {
		return RewriteFailed, nil
	}
	if len(lhs) != len(rhs) {
		return RewriteFailed, ast.ErrASTDomain
	}
	for i := range lhs {
		eq, e := m.MkEq(lhs[i], rhs[i])
		if e != nil {
			return RewriteFailed, e
		}
		if e = resV.pushBack(m, eq); e != nil {
			return RewriteFailed, e
		}
	}
	result, err := m.MkAndArray(resV.items)
	if err != nil {
		return RewriteFailed, err
	}
	if err = m.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return Rewrite3, nil
}
