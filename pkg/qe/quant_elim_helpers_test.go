package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// qeTestEliminate runs expr_quant_elim on one formula, as qe_tactic does,
// and reports whether every quantifier was eliminated.
func qeTestEliminate(m *ast.ASTManager, formula ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	truth, err := m.MkTrue()
	if err != nil {
		return result, false, err
	}
	qelim := newExprQuantElim(m, config)
	result, err = qelim.run(truth, formula)
	err = errors.Join(err, qelim.Close())
	if err != nil {
		if result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
		return ast.ASTRef{}, false, err
	}
	value, err := m.RefGet(result)
	if err != nil {
		return result, false, errors.Join(err, m.DelRef(&result))
	}
	quantified, err := m.HasQuantifiers(value)
	if err != nil {
		return result, false, errors.Join(err, m.DelRef(&result))
	}
	return result, !quantified, nil
}

func qeTestBoolRewriter(t *testing.T, f *fmFixture) *rewriter.BoolRewriter {
	t.Helper()
	rw, err := rewriter.NewBoolRewriter(f.r.manager, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, rw.Close()) })
	return rw
}
