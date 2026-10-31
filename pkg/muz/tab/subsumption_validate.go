package tab

// Source: src/muz/tab/tab_context.cpp, index::check_substitution. The solver
// borrows the query precondition in an enclosing scope.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func validateSubsumption(m *ast.ASTManager, candidate *Clause, bindings *substitution.Substitution, side []ast.ASTRef, lite *qe.Lite, smt solver.Solver) (valid bool, err error) {
	if m == nil || candidate == nil || candidate.closed || candidate.manager != m || bindings == nil || lite == nil || smt == nil {
		return false, ast.ErrASTHandle
	}
	solverManager, err := smt.Manager()
	if err != nil {
		return false, err
	}
	if solverManager != m {
		return false, ast.ErrASTHandle
	}
	if err = bindings.ResetCache(); err != nil {
		return false, err
	}
	var forms []ast.ASTRef
	defer func() {
		for i := len(forms) - 1; i >= 0; i-- {
			if forms[i] != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&forms[i]))
			}
		}
	}()
	apply := func(h ast.ASTHandle) error {
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		forms = append(forms, ref)
		return bindings.Apply(2, []uint32{0, 0}, substitution.NewExprOffset(h, 0), ref)
	}
	for _, ref := range side {
		h, e := m.RefGet(ref)
		if e != nil {
			return false, e
		}
		if e = apply(h); e != nil {
			return false, e
		}
	}
	constraint, err := candidate.Constraint()
	if err != nil {
		return false, err
	}
	if err = apply(constraint); err != nil {
		return false, err
	}
	if err = lite.EliminateConjunctions(nil, false, &forms); err != nil {
		return false, err
	}
	values := make([]ast.ASTHandle, len(forms))
	for i, ref := range forms {
		values[i], err = m.RefGet(ref)
		if err != nil {
			return false, err
		}
	}
	values, err = m.FlattenAnd(values)
	if err != nil {
		return false, err
	}
	post, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&post)) }()
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, values, post); err != nil {
		return false, err
	}
	h, err := m.RefGet(post)
	if err != nil {
		return false, err
	}
	if m.IsFalse(h) {
		return false, nil
	}
	if m.IsTrue(h) {
		return true, nil
	}
	ground, err := m.IsGround(h)
	if err != nil || !ground {
		return false, err
	}
	negated, err := m.MkNot(h)
	if err != nil {
		return false, err
	}
	if err = m.SetRef(post, negated); err != nil {
		return false, err
	}
	if err = smt.Push(); err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, smt.Pop(1)) }()
	if err = smt.Assert(negated); err != nil {
		return false, err
	}
	status, err := smt.CheckSat(nil)
	if err != nil {
		return false, err
	}
	return status == lbool.False, nil
}
