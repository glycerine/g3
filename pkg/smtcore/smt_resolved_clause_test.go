package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTScopedReducedBinaryReason(t *testing.T) {
	s, lits := sortedClauseInputs(t, 3)
	astTestMust(t, s.PushScope())
	first, second, removed := lits[0], lits[1], lits[2]
	astTestMust(t, s.AssignLiteral(removed.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, s.addClause(lits, true, ast.ASTHandle{}))
	clause := s.auxClauses[len(s.auxClauses)-1]
	reason, ok := clause.Justification().(*SMTUnitResolutionJustification)
	if !ok || !reason.InRegion() || reason.Antecedent != nil || len(reason.Literals) != 1 || reason.Literals[0] != removed.Negated() {
		t.Fatalf("lost resolved antecedent: %#v", reason)
	}
	if len(clause.literals) != 2 || cap(clause.literals) != 2 || s.ownedJustifications[len(s.ownedJustifications)-1] != reason {
		t.Fatal("wrong binary clause range or scope owner")
	}
	astTestMust(t, s.AssignLiteral(first.Negated(), NewSMTBooleanAxiom()))
	consistent, err := s.PropagateCore()
	astTestMust(t, err)
	got, err := s.Assignment(second)
	astTestMust(t, err)
	antecedent, found := s.booleanData[second.Var()].Justification().Clause()
	if !consistent || got != lbool.True || !found || antecedent != clause || reason.Literals[0] != removed.Negated() {
		t.Fatal("reduced clause did not propagate with its original reason")
	}
	astTestMust(t, s.PopScopes(1))
	if len(reason.Literals) != 0 || !clause.closed {
		t.Fatal("pop did not release the clause and its reason")
	}
}

func BenchmarkSMTScopedReducedBinaryClauses(b *testing.B) {
	s, lits := sortedClauseInputs(b, 3)
	const count = 64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		astTestMust(b, s.PushScope())
		astTestMust(b, s.AssignLiteral(lits[2].Negated(), NewSMTBooleanAxiom()))
		for j := 0; j < count; j++ {
			if err := s.addClause(lits, false, ast.ASTHandle{}); err != nil {
				b.Fatal(err)
			}
		}
		if len(s.auxClauses) != count {
			b.Fatal("wrong number of scoped clauses", len(s.auxClauses))
		}
		astTestMust(b, s.PopScopes(1))
	}
	b.ReportMetric(count, "clauses/op")
}
