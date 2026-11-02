package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityTryExpandNativeCollectiveConjectureAndDispatch(t *testing.T) {
	for _, mode := range []string{"proved", "failed", "disabled", "closed", "no-eligible", "proved-descendant-covered"} {
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
			p, err := ctx.BoolConst("try-expand-p")
			must(err)
			closeExpr(t, p)
			q, err := ctx.BoolConst("try-expand-q")
			must(err)
			closeExpr(t, q)
			for _, term := range []*Expr{p, q} {
				decl, err := term.Decl()
				must(err)
				must(source.DeclareConstant(decl))
				must(decl.Close())
			}
			relationTerm, err := ctx.BoolConst("try-expand-R")
			must(err)
			closeExpr(t, relationTerm)
			parentTerm, err := ctx.BoolConst("try-expand-parent")
			must(err)
			closeExpr(t, parentTerm)
			decl, err := relationTerm.Decl()
			must(err)
			defer decl.Close()
			relation, err := source.CreateNode(relationTerm)
			must(err)
			parent, err := source.CreateNode(parentTerm)
			must(err)
			parentRule, err := NewTransformer(ctx, []*FuncDecl{decl}, nil, relationTerm)
			must(err)
			defer parentRule.Close()
			parentEdge, err := source.CreateEdge(parent, parentRule, []*Node{relation})
			must(err)
			disjunction, err := p.Or(q)
			must(err)
			closeExpr(t, disjunction)
			truth, err := ctx.BoolVal(true)
			must(err)
			closeExpr(t, truth)
			formula := truth
			if mode == "proved" || mode == "proved-descendant-covered" {
				formula = disjunction
			}
			var childSource *Node
			var leafEdge *Edge
			var relations []*FuncDecl
			var sourceChildren []*Node
			if mode == "proved-descendant-covered" {
				leafTerm, err := ctx.BoolConst("try-expand-leaf")
				must(err)
				closeExpr(t, leafTerm)
				leafDecl, err := leafTerm.Decl()
				must(err)
				defer leafDecl.Close()
				childSource, err = source.CreateNode(leafTerm)
				must(err)
				relations = []*FuncDecl{leafDecl}
				sourceChildren = []*Node{childSource}
				formula, err = disjunction.And(leafTerm)
				must(err)
				closeExpr(t, formula)
				leafRule, err := NewTransformer(ctx, nil, nil, truth)
				must(err)
				defer leafRule.Close()
				leafEdge, err = source.CreateEdge(childSource, leafRule, nil)
				must(err)
			}
			rule, err := NewTransformer(ctx, relations, nil, formula)
			must(err)
			defer rule.Close()
			edge, err := source.CreateEdge(relation, rule, sourceChildren)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			must(u.CreateEdgesByChildMap())
			for _, fact := range []*Expr{p, q} {
				old, err := u.CreateNodeInstance(relation, 0)
				must(err)
				raw, err := fact.Raw()
				must(err)
				must(m.SetRef(old.Annotation.formula, raw))
				if mode == "no-eligible" {
					must(old.Annotation.SetEmpty())
				}
				_, err = graph.CreateLowerBoundEdge(old)
				must(err)
				must(u.ActivateInstance(old))
			}
			var children []*Node
			if childSource != nil {
				earlier, err := u.CreateNodeInstance(childSource, 0)
				must(err)
				_, err = u.CreateEdgeInstance(leafEdge, earlier, nil)
				must(err)
				must(u.ActivateInstance(earlier))
				child, err := u.CreateNodeInstance(childSource, 0)
				must(err)
				_, err = u.CreateEdgeInstance(leafEdge, child, nil)
				must(err)
				children = []*Node{child}
			}
			node, err := u.CreateNodeInstance(relation, 0)
			must(err)
			_, err = u.CreateEdgeInstance(edge, node, children)
			must(err)
			if mode == "closed" {
				raw, err := p.Raw()
				must(err)
				must(m.SetRef(node.Annotation.formula, raw))
			}
			working, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer working.Close()
			must(working.BindSolver(native))
			evaluator := derivationEvaluatorFixture(t, ctx, globals)
			must(working.BindModelEvaluator(evaluator))
			base, err := NewDerivationTree(graph, working)
			must(err)
			bookkeeping, err := NewBacktrackingDerivation(base)
			must(err)
			checker, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer checker.Close()
			search := &DerivationSearch{Search: bookkeeping, Checker: checker, Auxiliary: aux, Simplifier: manager}
			cex := NewCounterexample()
			defer cex.Close()
			services := &ExpansionSearch{Conjecture: search, Counterexample: cex, Native: native, Evaluator: evaluator, Simplifier: manager, NoConjectures: mode == "disabled"}
			must(native.Push())
			caller, err := ctx.BoolConst("try-expand-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			if mode == "closed" {
				must(u.TryExpandNode(node, nil))
			} else {
				must(u.TryExpandNode(node, services))
			}
			active := false
			for _, instance := range covering.instances[relation] {
				if instance == node {
					active = true
				}
			}
			switch mode {
			case "closed":
				if covering.coveredBy[node] == nil || active || working.closed || base.Top != nil {
					t.Fatal("cover shortcut ran conjecture/expansion")
				}
			case "proved":
				if covering.coveredBy[node] == nil || active || !working.closed {
					t.Fatal("proved collective conjecture was not covered")
				}
			case "proved-descendant-covered":
				if covering.coveredBy[node] != nil || active || !working.closed || covering.coveredBy[children[0]] == nil {
					t.Fatal("descendant-covered conjecture should not activate its ancestor")
				}
			default:
				if !active || !covering.dominated[covering.instances[relation][0]] {
					t.Fatal("failed/disabled conjecture did not add dominance and activate")
				}
				candidate, ok := u.NextCandidate()
				if !ok || candidate.Edge != parentEdge || len(candidate.Children) != 1 || candidate.Children[0].Owner != graph {
					t.Fatal("native expansion candidate missing")
				}
				if mode == "failed" {
					if !working.closed || cex.Tree() != nil {
						t.Fatal("failed conjecture leaked graph/result")
					}
				} else if working.closed || base.Top != nil {
					t.Fatal("unused conjecture graph consumed")
				}
			}
			if _, ok := u.NextCandidate(); ok {
				t.Fatal("unexpected candidate from closed/proved node or duplicate expansion")
			}
			if mode == "proved" || mode == "proved-descendant-covered" {
				// The saved bound preserves the source reverse-instance union order.
				bound, err := node.Bound.Formula()
				must(err)
				defer bound.Close()
				boundRaw, err := bound.Raw()
				must(err)
				no, err := m.MkFalse()
				must(err)
				qh, err := q.Raw()
				must(err)
				ph, err := p.Raw()
				must(err)
				first, err := m.MkOr(no, qh)
				must(err)
				firstRef, err := m.NewRef(first)
				must(err)
				defer m.DelRef(&firstRef)
				union, err := m.MkOr(first, ph)
				must(err)
				unionRef, err := m.NewRef(union)
				must(err)
				defer m.DelRef(&unionRef)
				yes, err := m.MkTrue()
				must(err)
				expected, err := m.MkAnd(yes, union)
				must(err)
				if boundRaw != expected {
					t.Fatal("collective conjecture changed reverse source union order")
				}
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("try-expand disturbed caller solver scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}
