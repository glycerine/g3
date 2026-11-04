package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityCandidateProductionNativePendingAndPostponedOrder(t *testing.T) {
	for _, mode := range []string{"pending", "global"} {
		t.Run(mode, func(t *testing.T) {
			setCaseInterpolationFaultSchedule(t, 4, 8)
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
			p, err := ctx.BoolConst("produce-p")
			must(err)
			closeExpr(t, p)
			pd, err := p.Decl()
			must(err)
			defer pd.Close()
			must(source.DeclareConstant(pd))
			childTerm, err := ctx.BoolConst("produce-child")
			must(err)
			closeExpr(t, childTerm)
			childDecl, err := childTerm.Decl()
			must(err)
			defer childDecl.Close()
			childSource, err := source.CreateNode(childTerm)
			must(err)
			value, err := ctx.BoolVal(mode == "pending")
			must(err)
			closeExpr(t, value)
			childRule, err := NewTransformer(ctx, nil, nil, value)
			must(err)
			defer childRule.Close()
			childEdge, err := source.CreateEdge(childSource, childRule, nil)
			must(err)
			var parentEdges []*Edge
			for _, name := range []string{"produce-parent0", "produce-parent1"} {
				term, err := ctx.BoolConst(name)
				must(err)
				closeExpr(t, term)
				parent, err := source.CreateNode(term)
				must(err)
				rule, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
				must(err)
				defer rule.Close()
				edge, err := source.CreateEdge(parent, rule, []*Node{childSource})
				must(err)
				parentEdges = append(parentEdges, edge)
			}
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			must(u.CreateEdgesByChildMap())
			var instances []*Node
			negative, err := p.Not()
			must(err)
			closeExpr(t, negative)
			for _, formula := range []*Expr{p, negative} {
				node, err := u.CreateNodeInstance(childSource, 0)
				must(err)
				raw, err := formula.Raw()
				must(err)
				must(m.SetRef(node.Annotation.formula, raw))
				_, err = u.CreateEdgeInstance(childEdge, node, nil)
				must(err)
				instances = append(instances, node)
				if mode == "global" {
					must(u.ActivateInstance(node))
				}
			}
			local, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer local.Close()
			global, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer global.Close()
			cex := NewCounterexample()
			defer cex.Close()
			cases := &CaseInterpolator{Graph: source}
			s := &UnwindingSearch{Unwinding: u, Native: native, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Local: local, Global: global, Auxiliary: aux, Simplifier: manager, Counterexample: cex, Cases: cases}
			must(native.Push())
			caller, err := ctx.BoolConst("produce-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			if mode == "pending" {
				must(u.FindNodesToExpand())
				if len(u.toExpand) != 2 || u.toExpand[0] != instances[0] || u.toExpand[1] != instances[1] {
					t.Fatal("pending queue source order")
				}
				must(s.ProduceSomeCandidates())
				if len(u.toExpand) != 1 || u.toExpand[0] != instances[1] || len(u.candidates) != 2 || len(u.PendingNodes()) != 1 {
					t.Fatal("production did not stop after first successful pending expansion")
				}
				heuristic := s.Heuristic
				heuristic.updates[instances[0]] = 7
				// A nonempty candidate queue leaves pending and postponed work untouched.
				must(s.ProduceCandidatesForExtension())
				if len(u.toExpand) != 1 || len(u.candidates) != 2 {
					t.Fatal("existing candidates did not take priority")
				}
				for _, edge := range parentEdges {
					candidate, ok := u.NextCandidate()
					if !ok || candidate.Edge != edge || candidate.Children[0] != instances[0] {
						t.Fatal("first pending expansion source order")
					}
				}
				must(s.ProduceSomeCandidates())
				if len(u.toExpand) != 0 || len(u.PendingNodes()) != 0 || s.Heuristic != heuristic || heuristic.updates[instances[0]] != 7 || cex.Tree() != nil {
					t.Fatal("fresh conjecture search lost shared heuristic, pending queue or graph ownership")
				}
				for _, edge := range parentEdges {
					candidate, ok := u.NextCandidate()
					if !ok || candidate.Edge != edge || candidate.Children[0] != instances[1] {
						t.Fatal("second pending expansion/conjecture failure did not generate correct candidates")
					}
				}
			} else {
				must(s.ProduceCandidatesForExtension())
				if len(u.candidates) != 1 || len(u.postponed) != 1 {
					t.Fatal("global candidates were not released one at a time")
				}
				must(s.ProduceCandidatesForExtension())
				if len(u.candidates) != 1 || len(u.postponed) != 1 {
					t.Fatal("nonempty queue consumed postponed work")
				}
				first, ok := u.NextCandidate()
				if !ok || first.Edge != parentEdges[0] {
					t.Fatal("first global candidate order")
				}
				if u.updatedNodes == nil {
					u.updatedNodes = make(map[*Node]bool)
				}
				u.updatedNodes[parentEdges[0].Parent] = true
				must(s.ProduceCandidatesForExtension())
				second, ok := u.NextCandidate()
				if !ok || second.Edge != parentEdges[1] || len(u.postponed) != 0 || !u.updatedNodes[parentEdges[0].Parent] {
					t.Fatal("postponed release rescanned or changed global order")
				}
			}
			if _, ok := u.NextCandidate(); ok {
				t.Fatal("unexpected extra candidate")
			}
			if cases.Graph != source || caseInterpolationFaultSchedule.count != 4 || caseInterpolationFaultSchedule.limit != 8 {
				t.Fatal("query lifecycle lost borrowed case graph or engine failure schedule")
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("candidate production disturbed caller solver scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}

func TestDualityFindPendingCandidatesFiltersWithoutClearingQueue(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("pending-filter-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	var nodes []*Node
	for _, number := range []int{-3, -2, -1, 0} {
		node, err := u.CreateNodeInstance(relation, number)
		must(err)
		_, err = graph.CreateLowerBoundEdge(node)
		must(err)
		nodes = append(nodes, node)
	}
	covering.coveredBy[nodes[0]] = nodes[0]
	must(covering.SetDominated(nodes[1]))
	u.toExpand = []*Node{nodes[3]}
	must(u.FindNodesToExpand())
	must(u.FindNodesToExpand())
	want := []*Node{nodes[3], nodes[2], nodes[3], nodes[2], nodes[3]}
	if len(u.toExpand) != len(want) {
		t.Fatal("pending append/dedup semantics")
	}
	for i, node := range want {
		if u.toExpand[i] != node {
			t.Fatal("pending number/filter order", i)
		}
	}
}
