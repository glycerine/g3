package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQEBoolSearchMatchesBranchExpansion(t *testing.T) {
	for _, tc := range []struct {
		name          string
		uninterpreted bool
	}{{"xor", false}, {"predicate", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			k := qsatKernelOwner(t, f)
			solver, err := k.solver()
			fmMust(t, err)
			p := f.constant("search-p", f.boolean)
			r := f.constant("search-free", f.boolean)
			truth := f.own(m.MkTrue())
			falsity := f.own(m.MkFalse())
			formula := f.own(m.MkXor(p, r))
			expected := truth
			if tc.uninterpreted {
				formula = f.predicate("search-predicate", p)
				positive := f.predicate("search-predicate", truth)
				negative := f.predicate("search-predicate", falsity)
				expected = f.own(m.MkOr(positive, negative))
			}
			fmMust(t, solver.AssertFormula(formula))
			context, err := NewSolverContext(m, []ast.ASTHandle{p})
			fmMust(t, err)
			defer func() { fmMust(t, context.Close()) }()
			plugin, err := NewBoolPlugin(context)
			fmMust(t, err)
			fmMust(t, context.AddPlugin(plugin))
			adapter, err := NewSolverNNFAdapter(context)
			fmMust(t, err)
			defer func() { fmMust(t, adapter.Close()) }()
			nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, nnf.Close()) }()
			rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
			fmMust(t, err)
			defer func() { fmMust(t, rw.Close()) }()
			partitioner, err := NewConjunctionPartitioner(m)
			fmMust(t, err)
			defer func() { fmMust(t, partitioner.Close()) }()
			fmMust(t, partitioner.AddPlugin(plugin))
			variables, err := newExprQEBranchVariables(m)
			fmMust(t, err)
			defer func() { fmMust(t, variables.Close()) }()
			fmMust(t, variables.add(p))
			root, err := newExprQETree(m, f.r.config, nil, truth)
			fmMust(t, err)
			defer func() { fmMust(t, root.Close()) }()
			fmMust(t, root.initialize(formula))
			fmMust(t, variables.consume(root))
			fmMust(t, normalizeExprQETree(root, rw, nnf))
			result, complete, err := searchExprQEScalarTree(root, variables, context, plugin, partitioner, solver, f.r.config, rw, nnf)
			fmMust(t, err)
			if !complete {
				t.Fatal("Boolean search did not finish")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			iff := f.own(m.MkIff(value, expected))
			different := f.own(m.MkNot(iff))
			check, err := k.solver()
			fmMust(t, err)
			fmMust(t, check.AssertFormula(different))
			status, err := check.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("search result differs from explicit Boolean expansion")
			}
		})
	}
}
