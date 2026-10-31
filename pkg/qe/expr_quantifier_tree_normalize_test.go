package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQETreeNormalizeRefreshesAtoms(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("normalize-p", f.boolean)
	q := f.constant("normalize-q", f.boolean)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{p, q})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	adapter, err := NewSolverNNFAdapter(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	node, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, node.Close()) }()
	formula := f.own(m.MkImplies(p, q))
	fmMust(t, node.initialize(formula))
	fmMust(t, normalizeExprQETree(node, rw, nnf))
	h, err := m.RefGet(node.formula)
	fmMust(t, err)
	assertQENNFConnectives(t, m, h)
	negative, err := node.atoms.Negative(p)
	fmMust(t, err)
	positive, err := node.atoms.Positive(q)
	fmMust(t, err)
	if !negative || !positive {
		t.Fatal("missing normalized atom polarities")
	}
	fmMust(t, node.initialize(p))
	fmMust(t, normalizeExprQETree(node, rw, nnf))
	negative, err = node.atoms.Negative(p)
	fmMust(t, err)
	positive, err = node.atoms.Positive(p)
	fmMust(t, err)
	if negative || !positive {
		t.Fatal("normalization left stale atom state")
	}
}
