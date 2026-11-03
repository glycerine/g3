package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// The constrained completion path must classify the engine's background
// assertions as interpolation theory. The graph is reachable by itself, but
// the background axiom refutes its root constraint.
func TestDualityFastDerivationThreadsBackgroundTheory(t *testing.T) {
	for _, withTheory := range []bool{false, true} {
		name := "without-theory"
		if withTheory {
			name = "with-theory"
		}
		t.Run(name, func(t *testing.T) {
			ctx, m, main, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer func() { must(manager.Close()) }()

			p, err := ctx.BoolConst("fast-theory-p")
			must(err)
			closeExpr(t, p)
			pDecl, err := p.Decl()
			must(err)
			closeDecl(t, pDecl)
			notP, err := p.Not()
			must(err)
			closeExpr(t, notP)
			truth, err := ctx.BoolVal(true)
			must(err)
			closeExpr(t, truth)

			source, err := NewRPFP(ctx)
			must(err)
			defer func() { must(source.Close()) }()
			must(source.DeclareConstant(pDecl))
			rootTerm, err := ctx.BoolConst("fast-theory-root")
			must(err)
			closeExpr(t, rootTerm)
			leafTerm, err := ctx.BoolConst("fast-theory-leaf")
			must(err)
			closeExpr(t, leafTerm)
			leafDecl, err := leafTerm.Decl()
			must(err)
			closeDecl(t, leafDecl)
			root, err := source.CreateNode(rootTerm)
			must(err)
			leaf, err := source.CreateNode(leafTerm)
			must(err)
			must(root.Bound.SetEmpty())
			rootFormula, err := notP.And(leafTerm)
			must(err)
			closeExpr(t, rootFormula)
			rootRule, err := NewTransformer(ctx, []*FuncDecl{leafDecl}, nil, rootFormula)
			must(err)
			defer func() { must(rootRule.Close()) }()
			leafRule, err := NewTransformer(ctx, nil, nil, truth)
			must(err)
			defer func() { must(leafRule.Close()) }()
			rootEdge, err := source.CreateEdge(root, rootRule, []*Node{leaf})
			must(err)
			leafEdge, err := source.CreateEdge(leaf, leafRule, nil)
			must(err)

			unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer func() { must(unwinding.Close()) }()
			mappedRoot, err := unwinding.CloneNode(root)
			must(err)
			mappedLeaf, err := unwinding.CloneNode(leaf)
			must(err)
			mappedRootEdge, err := unwinding.CreateEdge(mappedRoot, rootRule, []*Node{mappedLeaf})
			must(err)
			mappedRootEdge.Map = rootEdge
			mappedLeafEdge, err := unwinding.CreateEdge(mappedLeaf, leafRule, nil)
			must(err)
			mappedLeafEdge.Map = leafEdge

			working, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer func() { must(working.Close()) }()
			must(working.BindSolver(main))
			must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			base, err := NewDerivationTree(unwinding, working)
			must(err)
			bookkeeping, err := NewBacktrackingDerivation(base)
			must(err)
			controller := &DerivationSearch{Search: bookkeeping, Simplifier: manager}
			if withTheory {
				controller.Cases = &CaseInterpolator{Theory: []*Expr{p}}
				must(main.Push())
				defer func() { must(main.Pop(1)) }()
				h, err := p.Raw()
				must(err)
				must(main.Assert(h))
				status, err := main.CheckSat(nil)
				must(err)
				if status != lbool.True {
					t.Fatal("background axiom is inconsistent", status)
				}
			}

			sat, err := controller.Derive(mappedRoot, false, true)
			must(err)
			if sat == withTheory {
				t.Fatal("fast derivation ignored background theory", sat, withTheory)
			}
			if len(bookkeeping.stack) != 1 {
				t.Fatal("fast derivation left expansion scopes", len(bookkeeping.stack))
			}
			if withTheory {
				learned, err := base.Top.Annotation.Formula()
				must(err)
				h, err := learned.Raw()
				must(err)
				must(main.Push())
				must(main.Assert(h))
				status, err := main.CheckSat(nil)
				must(err)
				must(main.Pop(1))
				must(learned.Close())
				if status != lbool.False {
					t.Fatal("learned fast annotation admits the refuted background", status)
				}
			}
			level, err := main.ScopeLevel()
			must(err)
			wantLevel := uint32(0)
			if withTheory {
				wantLevel = 1
			}
			if level != wantLevel {
				t.Fatal("fast derivation changed caller scope", level, wantLevel)
			}
		})
	}
}
