package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type solverMutationFixture struct {
	t             *testing.T
	base          *fmFixture
	context       *SolverContext
	x, y, z, form ast.ASTHandle
}

func newSolverMutationFixture(t *testing.T) *solverMutationFixture {
	t.Helper()
	base := newFMFixture(t)
	x := base.constant("solver-mutation-x", base.integer)
	y := base.constant("solver-mutation-y", base.integer)
	z := base.constant("solver-mutation-z", base.integer)
	form := base.own(base.r.manager.MkEq(x, y))
	context, err := NewSolverContext(base.r.manager, []ast.ASTHandle{x, y, z})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, context.Close()) })
	return &solverMutationFixture{t: t, base: base, context: context, x: x, y: y, z: z, form: form}
}
