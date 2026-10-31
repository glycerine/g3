package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQEBoolBranchPropagatesThenSubstitutes(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("step-p", f.boolean)
	q := f.constant("step-q", f.boolean)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{p, q})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewBoolPlugin(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
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
	fmMust(t, variables.add(q))
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	formula := f.own(m.MkOr(p, q))
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	fmMust(t, root.selectVariable(p, big.NewRat(2, 1)))
	child, status, err := stepExprQEScalarBranch(root, variables, plugin, big.NewRat(1, 1), f.r.config, rw, nnf, nil, true)
	fmMust(t, err)
	if status != exprQENeedPropagation || child.formula != (ast.ASTRef{}) {
		t.Fatal("new branch was not left for propagation")
	}
	child2, status, err := stepExprQEScalarBranch(root, variables, plugin, big.NewRat(1, 1), f.r.config, rw, nnf, nil, true)
	fmMust(t, err)
	if child2 != child || status != exprQEChooseVariable {
		t.Fatal("existing branch not initialized")
	}
	h, err := m.RefGet(child.formula)
	fmMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("Boolean branch formula not simplified")
	}
	size, err := child.definitions.Size()
	fmMust(t, err)
	if size != 1 {
		t.Fatal("missing branch definition")
	}
	definition, err := child.definitions.Definition(0)
	fmMust(t, err)
	if !m.IsTrue(definition) {
		t.Fatal("wrong Boolean branch definition")
	}
	if len(child.variables) != 1 {
		t.Fatal("unselected variable lost")
	}
}

func TestExprQEBVBranchPropagatesThenSubstitutes(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	sort := f.own(m.BitVectorSort(2))
	x := f.constant("step-bv", sort)
	two := f.own(m.BitVectorNumeralUint64(2, 2))
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewBitVectorPlugin(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
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
	fmMust(t, variables.add(x))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	formula := f.own(m.MkEq(x, two))
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	fmMust(t, root.selectVariable(x, big.NewRat(4, 1)))
	child, status, err := stepExprQEScalarBranch(root, variables, plugin, big.NewRat(2, 1), f.r.config, rw, nnf, nil, true)
	fmMust(t, err)
	if status != exprQENeedPropagation || child.formula != (ast.ASTRef{}) {
		t.Fatal("missing propagation step")
	}
	child, status, err = stepExprQEScalarBranch(root, variables, plugin, big.NewRat(2, 1), f.r.config, rw, nnf, nil, true)
	fmMust(t, err)
	if status != exprQEChooseVariable {
		t.Fatal("missing substitution step")
	}
	h, err := m.RefGet(child.formula)
	fmMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("bit-vector branch did not simplify")
	}
	definition, err := child.definitions.Definition(0)
	fmMust(t, err)
	if definition != two {
		t.Fatal("wrong bit-vector branch definition")
	}
}
