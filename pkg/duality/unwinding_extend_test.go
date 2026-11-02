package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityExtendNativeEagerLazyCoveringAndCounterexample(t *testing.T) {
	for _, mode := range []string{"sat", "unsat", "eager", "underapprox", "full-bound", "completion-error"} {
		t.Run(mode, func(t *testing.T) {
			ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
			must(err)
			defer factory.Close()
			aux, err := factory.New(m, nil, false, true, false, "")
			must(err)
			defer aux.Close()
			manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer manager.Close()
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			p, err := ctx.BoolConst("extend-p")
			must(err)
			closeExpr(t, p)
			pd, err := p.Decl()
			must(err)
			defer pd.Close()
			must(source.DeclareConstant(pd))
			parentTerm, err := ctx.BoolConst("extend-parent")
			must(err)
			closeExpr(t, parentTerm)
			childTerm, err := ctx.BoolConst("extend-child")
			must(err)
			closeExpr(t, childTerm)
			childDecl, err := childTerm.Decl()
			must(err)
			defer childDecl.Close()
			parent, err := source.CreateNode(parentTerm)
			must(err)
			childSource, err := source.CreateNode(childTerm)
			must(err)
			if mode != "full-bound" {
				must(parent.Bound.SetEmpty())
			}
			rootFormula := childTerm
			if mode == "unsat" {
				negative, err := p.Not()
				must(err)
				closeExpr(t, negative)
				rootFormula, err = negative.And(childTerm)
				must(err)
				closeExpr(t, rootFormula)
			}
			if mode == "eager" {
				rootFormula, err = p.And(childTerm)
				must(err)
				closeExpr(t, rootFormula)
			}
			parentRule, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, rootFormula)
			must(err)
			defer parentRule.Close()
			sourceEdge, err := source.CreateEdge(parent, parentRule, []*Node{childSource})
			must(err)
			leafRule, err := NewTransformer(ctx, nil, nil, p)
			must(err)
			defer leafRule.Close()
			leafEdge, err := source.CreateEdge(childSource, leafRule, nil)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			outer, err := NewUnwinding(source, graph, covering)
			must(err)
			must(outer.DoTopoSort())
			if mode == "unsat" || mode == "full-bound" {
				earlier, err := outer.CreateNodeInstance(childSource, 0)
				must(err)
				_, err = outer.CreateEdgeInstance(leafEdge, earlier, nil)
				must(err)
				must(outer.ActivateInstance(earlier))
			}
			child, err := outer.CreateNodeInstance(childSource, 0)
			must(err)
			_, err = outer.CreateEdgeInstance(leafEdge, child, nil)
			must(err)
			outer.backEdges[child] = map[*Node]int{childSource: 2}
			under := mode == "underapprox" || mode == "completion-error"
			if under {
				must(child.Underapprox.SetFull())
			}
			checker, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer checker.Close()
			makeSearch := func() (*DerivationSearch, *RPFP) {
				working, err := NewRPFPWithVocabulary(source.vocabulary)
				must(err)
				t.Cleanup(func() { must(working.Close()) })
				must(working.BindSolver(native))
				must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
				base, err := NewDerivationTree(graph, working)
				must(err)
				bookkeeping, err := NewBacktrackingDerivation(base)
				must(err)
				return &DerivationSearch{Search: bookkeeping, Checker: checker, Auxiliary: aux, Simplifier: manager}, working
			}
			bounds, partial := makeSearch()
			completion, full := makeSearch()
			cex := NewCounterexample()
			defer cex.Close()
			prior, err := NewRPFP(ctx)
			must(err)
			priorRoot, err := prior.CreateNode(parentTerm)
			must(err)
			must(cex.Set(prior, priorRoot))
			services := &ExtensionSearch{Bounds: bounds, Completion: completion, Counterexample: cex, UseUnderapprox: under}
			if mode == "completion-error" {
				services.Completion = nil
			}
			if mode == "eager" {
				history, err := NewRPFP(ctx)
				must(err)
				defer history.Close()
				must(history.DeclareConstant(pd))
				old, err := history.CreateNode(parentTerm)
				must(err)
				ph, err := p.Raw()
				must(err)
				must(m.SetRef(old.Annotation.formula, ph))
				proposer, err := NewHistoryProposer(history, source)
				must(err)
				defer proposer.Close()
				services.Proposers = []*HistoryProposer{proposer}
			}
			must(native.Push())
			caller, err := ctx.BoolConst("extend-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			node, satisfied, err := outer.Extend(ExpansionCandidate{Edge: sourceEdge, Children: []*Node{child}}, services)
			if mode == "completion-error" {
				if !errors.Is(err, ErrWrapperOwner) {
					t.Fatal("completion failure missing", err)
				}
			} else {
				must(err)
			}
			if node == nil || node.Map != parent || node.Outgoing.Map != sourceEdge || node.Outgoing.Children[0] != child || outer.backEdges[node][childSource] != 2 {
				t.Fatal("extension graph/mapping/recursion counts incorrect")
			}
			wantSatisfied := mode == "unsat" || mode == "full-bound"
			if satisfied != wantSatisfied {
				t.Fatal("extension result", satisfied, mode)
			}
			if wantSatisfied && covering.coveredBy[child] == nil {
				t.Fatal("successful upper-bound solving did not close descendants")
			}
			if !wantSatisfied && covering.coveredBy[child] != nil {
				t.Fatal("SAT extension unexpectedly closed descendants")
			}
			switch mode {
			case "full-bound":
				if cex.Tree() != prior || prior.closed || partial.closed || full.closed || bounds.Search.Top != nil {
					t.Fatal("shortcut consumed unused services or old counterexample")
				}
			case "unsat":
				if cex.Tree() != prior || prior.closed || !partial.closed || full.closed {
					t.Fatal("refutation ownership")
				}
				empty, err := node.Annotation.SubsetEq(node.Bound, aux)
				must(err)
				if !empty {
					t.Fatal("refutation fact missing")
				}
			case "underapprox":
				if !prior.closed || !partial.closed || cex.Tree() != full || full.closed || len(full.edges) != 2 || !completion.Search.Constrained {
					t.Fatal("full counterexample not completed/transferred")
				}
			default:
				if !prior.closed || cex.Tree() != partial || partial.closed || full.closed {
					t.Fatal("SAT or completion-error lost partial counterexample")
				}
			}
			if mode == "eager" {
				fact, err := NewTransformer(ctx, nil, nil, p)
				must(err)
				defer fact.Close()
				learned, err := node.Annotation.SubsetEq(fact, aux)
				must(err)
				if !learned || checker.pool.states[sourceEdge] == nil {
					t.Fatal("eager history deduction was not connected")
				}
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("extension disturbed caller solver", depth, assertions, baseline)
			}
			sourceFull, err := parent.Annotation.IsFull()
			must(err)
			if !sourceFull {
				t.Fatal("extension mutated source annotation")
			}
			must(native.Pop(1))
		})
	}
}
