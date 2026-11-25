package smtcore

import (
	"errors"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTInvariantRejectsEmptyConcreteDirectoryEntry(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	s.auxClauses = append(s.auxClauses, nil)
	err := s.CheckInvariant()
	s.auxClauses = s.auxClauses[:len(s.auxClauses)-1]
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("empty entry returned %v", err)
	}
}

func TestSMTRetiredDirectorySnapshotSurvivesRefusedPop(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, _, s := smtInternalizerFixture(t, m, nil)
	var atoms []ast.ASTHandle
	for _, n := range []string{"record-a", "record-b", "record-c", "record-d"} {
		atoms = append(atoms, smtRelevancyBool(t, m, n))
	}
	ls := scopeRestorationLiterals(t, s, atoms...)
	proof, e := m.MkProofAsserted(atoms[0])
	astTestMust(t, e)
	astTestPin(t, m, proof)
	smtTestUserPush(t, s)
	astTestMust(t, s.addClause(ls[:3], false, proof))
	astTestMust(t, s.addClause(ls[1:], true, proof))
	old, newer := s.testClauses()[0].clause, s.testClauses()[1].clause
	expected := s.Clauses()
	// A public caller may shorten the live object before the solver changes its directory.
	// The invariant must still compare the two widths. A later, interrupted pop publishes
	// the original literal range for the record that has already been retired.
	astTestMust(t, newer.SetNumLiterals(2))
	if e = s.CheckInvariant(); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("directory lost original width", e)
	}
	if got := s.Clauses()[1].Literals; !slices.Equal(got, expected[1].Literals[:2]) {
		t.Fatal("live snapshot did not use shortened clause", got)
	}
	lit := old.literals[1].Negated()
	if !s.watches[lit.Index()].RemoveClause(old) {
		t.Fatal("missing second watch")
	}
	if e = s.PopScopes(1); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("pop did not refuse missing watch", e)
	}
	snapshot := s.Clauses()
	if len(snapshot) != 2 || !newer.closed || newer.Literals() != nil || !snapshot[1].Root || snapshot[1].Proof != proof || !slices.Equal(snapshot[1].Literals, expected[1].Literals) {
		t.Fatal("retired directory lost original literals, root or proof", snapshot)
	}
	snapshot[1].Literals[0] = SMTNullLiteral
	if !slices.Equal(s.Clauses()[1].Literals, expected[1].Literals) {
		t.Fatal("snapshot aliases solver storage")
	}
	astTestMust(t, s.watches[lit.Index()].InsertClause(old))
	astTestMust(t, s.PopScopes(1))
	if len(s.Clauses()) != 0 {
		t.Fatal("retry kept concrete records")
	}
}
