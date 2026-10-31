package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQETreeChooseBranchSelectsAndPublishes(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("choose-p", f.boolean)
	truth := f.own(m.MkTrue())
	context, err := NewSolverContext(m, []ast.ASTHandle{p})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	plugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(p))
	fmMust(t, variables.consume(root))
	root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{p}, nil)
	fmMust(t, err)
	var free []ast.ASTRef
	defer func() {
		for i := range free {
			fmMust(t, m.DelRef(&free[i]))
		}
	}()
	found, err := chooseExprQETreeBranch(root, variables, context, solver, &free)
	fmMust(t, err)
	if !found || root.selected == (ast.ASTRef{}) || root.branches.RatString() != "2" || len(root.variables) != 0 || len(free) != 0 {
		t.Fatal("Boolean branch was not selected and constrained")
	}
	selected, err := m.RefGet(root.selected)
	fmMust(t, err)
	if selected != p {
		t.Fatal("wrong variable selected")
	}
}
