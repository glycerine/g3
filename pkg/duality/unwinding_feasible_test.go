package duality

import (
	"bytes"
	"github.com/glycerine/g3/pkg/interp"
	"strings"
	"testing"
)

func TestCandidateFeasibilityUsesCurrentSolutionAndRestoresNestedSolver(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	term, err := ctx.BoolConst("feasible-R")
	must(err)
	closeExpr(t, term)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	trueEdge, err := source.CreateLowerBoundEdge(relation)
	must(err)
	falseF, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer falseF.Close()
	falseEdge, err := source.CreateEdge(relation, falseF, nil)
	must(err)
	loopEdge, err := source.CreateEdge(relation, relation.Annotation, []*Node{relation})
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	var report bytes.Buffer
	graph.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: &report, Params: ctx.config.params, Globals: globals}, depth: -1}
	covering, err := NewCovering(graph, s)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	instance, err := u.CreateNodeInstance(relation, 0)
	must(err)
	_, err = u.CreateEdgeInstance(trueEdge, instance, nil)
	must(err)
	must(s.Push())
	h, err := term.Raw()
	must(err)
	must(s.Assert(h))
	check := func(edge *Edge, children []*Node, want bool) {
		t.Helper()
		got, err := u.CandidateFeasible(ExpansionCandidate{Edge: edge, Children: children}, s, simplifier)
		must(err)
		if got != want {
			t.Fatal("feasibility", got, want)
		}
		level, err := s.ScopeLevel()
		must(err)
		count, err := s.NumAssertions()
		must(err)
		if level != 1 || count != 1 {
			t.Fatal("temporary checker changed caller scope", level, count)
		}
	}
	check(trueEdge, nil, true)
	check(falseEdge, nil, false)
	if !strings.Contains(report.String(), "]reject ") {
		t.Fatal("missing live rejection event", report.String())
	}
	must(u.ActivateInstance(instance))
	check(trueEdge, nil, false) // Current solution already covers true.
	must(covering.SetInstanceActive(instance, false))
	second, err := u.CreateNodeInstance(relation, 0)
	must(err)
	_, err = u.CreateEdgeInstance(loopEdge, second, []*Node{instance})
	must(err)
	third, err := u.CreateNodeInstance(relation, 0)
	must(err)
	_, err = u.CreateEdgeInstance(loopEdge, third, []*Node{second})
	must(err)
	order := map[*Node]int{relation: 0}
	must(u.UpdateBackEdges(second, order))
	must(u.UpdateBackEdges(third, order))
	relation.RecursionBound = 1
	u.RecursionBound = 1
	past, err := u.NodePastRecursionBound(second)
	must(err)
	if past {
		t.Fatal("bound equality was cut off")
	}
	past, err = u.NodePastRecursionBound(third)
	must(err)
	if !past {
		t.Fatal("deeper recursion not cut off")
	}
	check(loopEdge, []*Node{third}, false)
	u.RecursionBound = -1
	check(loopEdge, []*Node{third}, true)
	u.FeasibleEdges = false
	check(falseEdge, nil, true)
	must(s.Pop(1))
}
