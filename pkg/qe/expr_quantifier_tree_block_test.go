package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQETreeBlocksOnlySolvedPath(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("block-p", f.boolean)
	truth := f.own(m.MkTrue())
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, root.consumeVariables([]ast.ASTHandle{p}))
	blocked, err := blockExprQETreeIfSolved(root, solver)
	fmMust(t, err)
	if blocked {
		t.Fatal("unfinished node was blocked")
	}
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("unfinished node changed the solver")
	}
	fmMust(t, root.deleteVariable(p))
	blocked, err = blockExprQETreeIfSolved(root, solver)
	fmMust(t, err)
	if !blocked {
		t.Fatal("finished node was not blocked")
	}
	status, err = solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("root assignment was not blocked")
	}
}
