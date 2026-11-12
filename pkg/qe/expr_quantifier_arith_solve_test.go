package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// TestExprQEArithmeticSolvePublishesFreshVariable follows
// arith_qe_util::solve_linear through quant_elim_plugin's add_var and
// elim_var: 2x + 3y + 1 = 0 has no unit coefficient, so x is replaced by a
// definition over a fresh integer that joins the unit branch's variables.
func TestExprQEArithmeticSolvePublishesFreshVariable(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("equation-x", f.integer)
	y := f.constant("equation-y", f.integer)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x, y})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewArithmeticPlugin(ctx, f.r.config)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(plugin))
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	threeY := f.own(m.MkArithmeticMul(f.number("3"), y))
	linear := f.own(m.MkArithmeticAdd(twoX, threeY))
	left := f.own(m.MkArithmeticAdd(linear, f.number("1")))
	formula := f.own(m.MkEq(left, f.number("0")))
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
	adapter, err := NewSolverNNFAdapter(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	tree := &exprQETreeContext{current: root, variables: vars, config: f.r.config, rw: rw, nnf: nnf}
	conjs, err := NewConjunctionEnum(m, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	ctx.tree = tree
	solved, err := plugin.Solve(conjs, formula)
	ctx.tree = nil
	fmMust(t, err)
	if !solved {
		t.Fatal("equation was not solved")
	}
	child := tree.current
	if !root.isUnit() || len(child.variables) != 2 || len(vars.pending) != 0 {
		t.Fatal("fresh variable not published in unit branch")
	}
	fresh, err := m.RefGet(child.variables[1])
	fmMust(t, err)
	if fresh == x || fresh == y {
		t.Fatal("unit branch lacks the fresh variable")
	}
	if _, ok := vars.index[fresh]; !ok {
		t.Fatal("fresh variable was not added to the branch tables")
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
