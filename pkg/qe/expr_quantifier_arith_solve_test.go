package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQEArithmeticSolvePublishesFreshVariable(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("equation-x", f.integer)
	y := f.constant("equation-y", f.integer)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x, y})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewArithmeticPlugin(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	threeY := f.own(m.MkArithmeticMul(f.number("3"), y))
	linear := f.own(m.MkArithmeticAdd(twoX, threeY))
	left := f.own(m.MkArithmeticAdd(linear, f.number("1")))
	formula := f.own(m.MkEq(left, f.number("0")))
	conjs, err := NewConjunctionEnum(m, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	solved, err := plugin.Solve(conjs, formula)
	fmMust(t, err)
	if !solved {
		t.Fatal("equation was not solved")
	}
	result, err := plugin.TakeEquationElimination()
	fmMust(t, err)
	if result == nil || len(result.Fresh) != 1 {
		t.Fatal("expected fresh integer result")
	}
	defer func() { fmMust(t, result.Close()) }()
	fresh, err := m.RefGet(result.Fresh[0])
	fmMust(t, err)
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(y))
	fmMust(t, vars.add(x))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, vars.consume(root))
	normCtx, err := NewSolverContext(m, []ast.ASTHandle{y, fresh})
	fmMust(t, err)
	defer func() { fmMust(t, normCtx.Close()) }()
	adapter, err := NewSolverNNFAdapter(normCtx)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	child, err := applyExprQEArithmeticResult(root, vars, ctx, result, f.r.config, rw, nnf)
	fmMust(t, err)
	if !root.isUnit() || len(child.variables) != 2 || len(vars.pending) != 0 {
		t.Fatal("fresh variable not published in unit branch")
	}
	for i, want := range []ast.ASTHandle{y, fresh} {
		got, e := m.RefGet(child.variables[i])
		fmMust(t, e)
		if got != want {
			t.Fatal("child variables", i)
		}
	}
	definition, err := child.definitions.Definition(0)
	fmMust(t, err)
	contains, err := ast.NewContainsApp(m, fresh)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	found, err := contains.Check(definition)
	fmMust(t, err)
	if !found {
		t.Fatal("definition lost fresh variable")
	}
}
