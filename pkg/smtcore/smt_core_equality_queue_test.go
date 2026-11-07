package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTCoreEqualityQueueCountsAsSearchProgress(t *testing.T) {
	m := astTestManager(t)
	_, values := smtUFConstants(t, m, "queued-search-progress", 2)
	_, _, s := smtInternalizerFixture(t, m, nil)
	for _, value := range values {
		astTestMust(t, s.Internalize(value, false))
	}
	left, right := s.context.enodes.get(values[0]), s.context.enodes.get(values[1])
	astTestMust(t, s.addEquality(left, right, NewSMTEqualityAxiom()))
	_, err := s.PropagateCore()
	astTestMust(t, err)
	base := s.searchProgress()
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, s.PushScope())
		astTestMust(t, s.assignEquality(left, right, NewSMTEqualityAxiom()))
		queued := s.searchProgress()
		if queued == base || !s.canPropagate() {
			t.Fatal("queued redundant equality was invisible to the search progress guard")
		}
		astTestMust(t, s.propagateCoreEqualities())
		consumed := s.searchProgress()
		if consumed == queued || s.canPropagate() {
			t.Fatal("consumed equality was invisible to the search progress guard")
		}
		astTestMust(t, s.propagateCoreEqualities())
		if s.searchProgress() != consumed {
			t.Fatal("an empty queue reported further search progress")
		}
		astTestMust(t, s.PopScopes(1))
		if s.searchProgress() != base {
			t.Fatal("queued search progress did not restore with its scope")
		}
	}
}

func TestSMTCoreEqualityQueueFIFOWithGeneratedCongruenceConflict(t *testing.T) {
	m := astTestManager(t)
	sort, values := smtUFConstants(t, m, "queued-core", 3)
	_, _, s := smtInternalizerFixture(t, m, nil)
	var nodes []*SMTEnode
	for _, value := range values {
		astTestMust(t, s.Internalize(value, false))
		nodes = append(nodes, s.context.enodes.get(value))
	}
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "queued-core-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	var applications []ast.ASTHandle
	for _, value := range values[:2] {
		term, err := m.MkApp(function, []ast.ASTHandle{value})
		astTestMust(t, err)
		astTestMust(t, s.Internalize(term, false))
		applications = append(applications, term)
	}
	equality, err := m.MkEq(applications[0], applications[1])
	astTestMust(t, err)
	astTestMust(t, s.Internalize(equality, false))
	literal, err := s.Literal(equality)
	astTestMust(t, err)
	left, right := s.context.enodes.get(applications[0]), s.context.enodes.get(applications[1])
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(literal.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, s.addDisequality(left, right, literal.Negated()))
	astTestMust(t, s.assignEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
	astTestMust(t, s.assignEquality(nodes[1], nodes[2], NewSMTEqualityAxiom()))
	if nodes[0].root == nodes[1].root || nodes[1].root == nodes[2].root {
		t.Fatal("assign_eq performed an eager merge")
	}
	astTestMust(t, s.propagateCoreEqualities())
	// The first merge appends f(a)=f(b) behind the already queued b=c.
	// Its conflict must therefore happen after b=c has been processed.
	if !s.Inconsistent() || nodes[0].root != nodes[1].root || nodes[1].root != nodes[2].root || left.root == right.root {
		t.Fatal("generated congruence did not preserve source FIFO order")
	}
	astTestMust(t, s.PopScopes(1))
	consistent, err := s.PropagateCore()
	astTestMust(t, err)
	if !consistent || nodes[0].root == nodes[1].root || nodes[1].root == nodes[2].root || s.context.isDisequal(left, right) {
		t.Fatal("queued equality/conflict state survived child pop")
	}
}

func TestSMTCoreEqualityQueueUndeliveredPopAndRedundantAccounting(t *testing.T) {
	m := astTestManager(t)
	_, values := smtUFConstants(t, m, "queued-rollback", 3)
	_, _, s := smtInternalizerFixture(t, m, nil)
	var nodes []*SMTEnode
	for _, value := range values {
		astTestMust(t, s.Internalize(value, false))
		nodes = append(nodes, s.context.enodes.get(value))
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.assignEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
	if !s.canPropagate() {
		t.Fatal("queued core equality did not request propagation")
	}
	astTestMust(t, s.PopScopes(1))
	if s.canPropagate() {
		t.Fatal("undelivered child equality survived pop")
	}
	astTestMust(t, s.PushScope())
	before := s.statistics.NumAddEq
	for _, pair := range [][2]int{{0, 1}, {1, 2}, {0, 2}} {
		astTestMust(t, s.assignEquality(nodes[pair[0]], nodes[pair[1]], NewSMTEqualityAxiom()))
	}
	astTestMust(t, s.propagateCoreEqualities())
	if s.statistics.NumAddEq != before+3 || nodes[0].root != nodes[2].root || s.canPropagate() {
		t.Fatal("source redundant queued equality was skipped or queue did not drain", s.statistics.NumAddEq-before)
	}
	astTestMust(t, s.PopScopes(1))
}
