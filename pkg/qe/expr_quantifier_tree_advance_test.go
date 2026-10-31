package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func TestExprQETreeAdvanceAppliesModelBranch(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("advance-p", f.boolean)
	q := f.constant("advance-q", f.boolean)
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
	ctx, err := NewSolverContext(m, []ast.ASTHandle{p})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewBoolPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(plugin))
	adapter, err := NewSolverNNFAdapter(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	formula := f.own(m.MkOr(p, q))
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{p}, nil)
	fmMust(t, err)
	var free []ast.ASTRef
	found, err := chooseExprQETreeBranch(root, variables, ctx, solver, &free)
	fmMust(t, err)
	if !found || len(free) != 0 {
		t.Fatal("model branch selection failed")
	}
	child, status, err := advanceExprQEScalarTree(root, variables, ctx, eval, f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if status != exprQENeedPropagation || child.formula != (ast.ASTRef{}) {
		t.Fatal("missing model-guided propagation")
	}
	child2, status, err := advanceExprQEScalarTree(root, variables, ctx, eval, f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if child2 != child || status != exprQEChooseVariable {
		t.Fatal("branch substitution did not follow propagation")
	}
	result, err := m.RefGet(child.formula)
	fmMust(t, err)
	if !m.IsTrue(result) {
		t.Fatal("model branch formula did not simplify")
	}
	again, status, err := advanceExprQEScalarTree(root, variables, ctx, eval, f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if again != child || status != exprQEChooseVariable {
		t.Fatal("initialized branch was not reused")
	}
}
