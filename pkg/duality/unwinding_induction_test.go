package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityMarkedSolutionsActivePendingCoveredAndBounded(t *testing.T) {
	ctx, m, aux, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("marked-solution-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	p, err := ctx.BoolConst("marked-solution-p")
	must(err)
	closeExpr(t, p)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	var instances []*Node
	for _, formula := range []*Expr{p, negative} {
		node, err := u.CreateNodeInstance(relation, 0)
		must(err)
		h, err := formula.Raw()
		must(err)
		must(m.SetRef(node.Annotation.formula, h))
		_, err = graph.CreateLowerBoundEdge(node)
		must(err)
		instances = append(instances, node)
	}
	must(u.ActivateInstance(instances[0]))
	a, err := NodeMarker(instances[0], nil)
	must(err)
	closeExpr(t, a)
	b, err := NodeMarker(instances[1], nil)
	must(err)
	closeExpr(t, b)
	destination, err := relation.Annotation.Clone()
	must(err)
	defer destination.Close()
	equivalent := func(want *Expr) {
		t.Helper()
		fact, err := NewTransformer(ctx, nil, nil, want)
		must(err)
		defer fact.Close()
		forward, err := destination.SubsetEq(fact, aux)
		must(err)
		back, err := fact.SubsetEq(destination, aux)
		must(err)
		if !forward || !back {
			t.Fatal("marked solution differs from expected Boolean meaning")
		}
	}
	available, err := u.GenNodeSolutionWithMarkers(relation, destination, true, nil, manager)
	must(err)
	onlyA, err := a.And(p)
	must(err)
	closeExpr(t, onlyA)
	if !available {
		t.Fatal("active instance unavailable")
	}
	equivalent(onlyA)
	available, err = u.GenNodeSolutionWithMarkers(relation, destination, false, nil, manager)
	must(err)
	na, err := a.Not()
	must(err)
	closeExpr(t, na)
	nb, err := b.Not()
	must(err)
	closeExpr(t, nb)
	guardA, err := na.Or(p)
	must(err)
	closeExpr(t, guardA)
	guardB, err := nb.Or(negative)
	must(err)
	closeExpr(t, guardB)
	either, err := a.Or(b)
	must(err)
	closeExpr(t, either)
	guards, err := guardA.And(guardB)
	must(err)
	closeExpr(t, guards)
	both, err := guards.And(either)
	must(err)
	closeExpr(t, both)
	if !available {
		t.Fatal("pending instances unavailable")
	}
	equivalent(both)
	pairMarker, err := NodeMarker(instances[0], instances[1])
	must(err)
	closeExpr(t, pairMarker)
	pairExpected, err := pairMarker.And(p)
	must(err)
	closeExpr(t, pairExpected)
	_, err = u.GenNodeSolutionWithMarkers(relation, destination, true, instances[1], manager)
	must(err)
	equivalent(pairExpected)
	u.RecursionBound = 0
	relation.RecursionBound = 0
	u.backEdges[instances[0]] = map[*Node]int{relation: 1}
	available, err = u.GenNodeSolutionWithMarkers(relation, destination, true, nil, manager)
	must(err)
	empty, err := destination.IsEmpty()
	must(err)
	if !available || !empty {
		t.Fatal("source availability must survive exclusion by recursion bound")
	}
	covering.coveredBy[instances[0]] = instances[0]
	available, err = u.GenNodeSolutionWithMarkers(relation, destination, true, nil, manager)
	must(err)
	if available {
		t.Fatal("covered instance contributed")
	}
}

func TestDualityInductionCheckerNativeMarkersAndEarlyReturns(t *testing.T) {
	for _, mode := range []string{"sat", "full-parent", "no-child", "past-bound"} {
		t.Run(mode, func(t *testing.T) {
			ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer manager.Close()
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			parentTerm, err := ctx.BoolConst("induction-parent")
			must(err)
			closeExpr(t, parentTerm)
			childTerm, err := ctx.BoolConst("induction-child")
			must(err)
			closeExpr(t, childTerm)
			declaration, err := childTerm.Decl()
			must(err)
			defer declaration.Close()
			parent, err := source.CreateNode(parentTerm)
			must(err)
			child, err := source.CreateNode(childTerm)
			must(err)
			rule, err := NewTransformer(ctx, []*FuncDecl{declaration}, nil, childTerm)
			must(err)
			defer rule.Close()
			edge, err := source.CreateEdge(parent, rule, []*Node{child})
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, native)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			instance, err := u.CreateNodeInstance(child, 0)
			must(err)
			_, err = graph.CreateLowerBoundEdge(instance)
			must(err)
			if mode != "no-child" {
				must(u.ActivateInstance(instance))
			}
			if mode == "full-parent" {
				earlier, err := u.CreateNodeInstance(parent, 0)
				must(err)
				_, err = graph.CreateLowerBoundEdge(earlier)
				must(err)
				must(u.ActivateInstance(earlier))
			}
			if mode == "past-bound" {
				u.RecursionBound = 0
				child.RecursionBound = 0
				u.backEdges[instance] = map[*Node]int{child: 1}
			}
			checker, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer checker.Close()
			must(checker.BindSolver(native))
			must(checker.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			must(native.Push())
			caller, err := ctx.BoolConst("induction-caller")
			must(err)
			closeExpr(t, caller)
			callerRaw, err := caller.Raw()
			must(err)
			must(native.Assert(callerRaw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			must(checker.Push())
			root, err := u.CheckerJustForEdge(edge, checker, true, manager)
			must(err)
			if mode == "full-parent" || mode == "no-child" {
				if root != nil {
					t.Fatal("source early return missing")
				}
			} else {
				if root == nil || root.Outgoing == nil || len(checker.edges) != 2 {
					t.Fatal("induction checker graph incomplete")
				}
				status, err := checker.Check(root, nil, nil)
				must(err)
				if mode == "past-bound" {
					if status != lbool.False {
						t.Fatal("excluded marker checker should be UNSAT", status)
					}
				} else {
					if status != lbool.True {
						t.Fatal(status)
					}
					marker, err := NodeMarker(instance, nil)
					must(err)
					defer marker.Close()
					lower := root.Outgoing.Children[0].Outgoing
					value, err := checker.Eval(lower, marker)
					must(err)
					defer value.Close()
					raw, err := value.Raw()
					must(err)
					if !m.IsTrue(raw) {
						t.Fatal("localized selection marker not forced")
					}
					localized, err := checker.Localize(lower, marker)
					must(err)
					defer localized.Close()
					negative, err := localized.Not()
					must(err)
					defer negative.Close()
					raw, err = negative.Raw()
					must(err)
					must(native.Assert(raw))
					status, err = checker.Check(root, nil, nil)
					must(err)
					if status != lbool.False {
						t.Fatal("marker omission should refute checker")
					}
				}
			}
			must(checker.Pop(1))
			depth, err := native.ScopeLevel()
			must(err)
			remaining, err := native.NumAssertions()
			must(err)
			if depth != 1 || remaining != baseline {
				t.Fatal("checker leaked caller scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}
