package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestExprQETreeSeekUsesModelAndUnitBranches(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("seek-p", f.boolean)
	truth := f.own(m.MkTrue())
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	decl, err := m.GetDecl(p)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(decl, truth))
	eval, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
	fmMust(t, err)
	defer func() { fmMust(t, eval.Close()) }()
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, vars.consume(root))
	fmMust(t, root.selectVariable(p, big.NewRat(2, 1)))
	current, status, branch, err := seekExprQETree(root, vars, eval)
	fmMust(t, err)
	if current != root || status != exprQENeedPropagation || branch.RatString() != "1" {
		t.Fatal("missing model branch")
	}
	child, err := root.addBranch(f.r.config, big.NewRat(1, 1), p)
	fmMust(t, err)
	current, status, _, err = seekExprQETree(root, vars, eval)
	fmMust(t, err)
	if current != child || status != exprQEChooseVariable {
		t.Fatal("uninitialized child not selected")
	}
	fmMust(t, child.initialize(truth))
	unit, err := child.addUnit(f.r.config, truth)
	fmMust(t, err)
	current, status, _, err = seekExprQETree(root, vars, eval)
	fmMust(t, err)
	if current != unit || status != exprQEChooseVariable {
		t.Fatal("unit branch was not traversed")
	}
	limited, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, limited.Close()) }()
	fmMust(t, limited.initialize(truth))
	fmMust(t, limited.consumeVariables([]ast.ASTHandle{p}))
	fmMust(t, limited.selectVariable(p, big.NewRat(1, 1)))
	_, status, branch, err = seekExprQETree(limited, vars, eval)
	fmMust(t, err)
	if status != exprQENeedPropagation || branch.Sign() != 0 {
		t.Fatal("out-of-range model branch did not default to zero")
	}
}
