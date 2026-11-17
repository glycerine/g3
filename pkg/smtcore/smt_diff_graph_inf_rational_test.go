package smtcore

import (
	"reflect"
	"testing"
)

func TestSMTInfRationalLexicographicOrder(t *testing.T) {
	zero := diffInf(0, 0)
	positiveEpsilon := diffInf(0, 1)
	negativeEpsilon := diffInf(0, -1)
	negativeRational := diffInf(-1, 100)
	if negativeRational.Cmp(negativeEpsilon) >= 0 || negativeEpsilon.Cmp(zero) >= 0 || zero.Cmp(positiveEpsilon) >= 0 {
		t.Fatal("infinitesimal rational order is not lexicographic")
	}
	copy := new(smtInfIntRational).Set(positiveEpsilon)
	positiveEpsilon.infinitesimal = 9
	if copy.infinitesimal != 1 {
		t.Fatal("infinitesimal rational copy aliases its source")
	}
}

func TestSMTInfRationalDiffGraphStrictNegativeCycle(t *testing.T) {
	g := NewSMTInfRationalDiffGraph(testQ())
	for variable := int32(0); variable < 2; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	weak := NewSMTLiteral(7, false)
	strict := NewSMTLiteral(8, true)
	first, err := g.AddEdge(0, 1, diffInf(0, 0), weak)
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.AddEdge(1, 0, diffInf(0, -1), strict)
	if err != nil {
		t.Fatal(err)
	}
	if feasible, enableErr := g.EnableEdge(first); enableErr != nil || !feasible {
		t.Fatal("weak edge", feasible, enableErr)
	}
	if feasible, enableErr := g.EnableEdge(second); enableErr != nil || feasible {
		t.Fatal("strict negative cycle", feasible, enableErr)
	}
	explanation, err := g.NegativeCycleExplanations()
	if err != nil || !reflect.DeepEqual(explanation, []SMTLiteral{strict, weak}) {
		t.Fatal("cycle explanation", explanation, err)
	}
	for variable := int32(0); variable < 2; variable++ {
		value, assignmentErr := g.testAssignment(variable)
		if assignmentErr != nil || value.Sign() != 0 {
			t.Fatal("failed activation changed assignment", variable, value, assignmentErr)
		}
	}
}

func TestSMTInfRationalDiffGraphAssignmentScopeAndNormalization(t *testing.T) {
	g := NewSMTInfRationalDiffGraph(testQ())
	for variable := int32(0); variable < 3; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	base, err := g.AddEdge(0, 1, diffInf(-2, -1), NewSMTLiteral(3, false))
	if err != nil {
		t.Fatal(err)
	}
	if feasible, enableErr := g.EnableEdge(base); enableErr != nil || !feasible {
		t.Fatal(feasible, enableErr)
	}
	if g.edges[base].timestamp != 0 || g.timestamp != 1 {
		t.Fatal("base enable timestamp", g.edges[base].timestamp, g.timestamp)
	}
	value, err := g.testAssignment(1)
	if err != nil || value.Cmp(diffInf(-2, -1)) != 0 {
		t.Fatal("assignment", value, err)
	}
	if err = g.Push(); err != nil {
		t.Fatal(err)
	}
	temporary, err := g.AddEdge(1, 2, diffInf(1, 0), NewSMTLiteral(4, false))
	if err != nil {
		t.Fatal(err)
	}
	if feasible, enableErr := g.EnableEdge(temporary); enableErr != nil || !feasible {
		t.Fatal(feasible, enableErr)
	}
	if g.edges[temporary].timestamp != 1 || g.timestamp != 2 {
		t.Fatal("scoped enable timestamp", g.edges[temporary].timestamp, g.timestamp)
	}
	if err = g.Pop(1); err != nil || g.NumEdges() != 1 {
		t.Fatal("scope rollback", g.NumEdges(), err)
	}
	if g.timestamp != 1 {
		t.Fatal("scope did not restore enable timestamp", g.timestamp)
	}
	if err = g.SetToZero(1); err != nil {
		t.Fatal(err)
	}
	value, err = g.testAssignment(1)
	if err != nil || value.Sign() != 0 || !g.IsFeasible() {
		t.Fatal("zero normalization", value, err)
	}
}
