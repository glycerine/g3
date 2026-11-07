package smtcore

import (
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// smtDiseqFixture builds enodes for count constants plus assumption literals
// used to justify merges, and returns the expanded conflict antecedents helper.
func smtDiseqFixture(t *testing.T, prefix string, count, literals int) (*ast.ASTManager, *SMTInternalizerState, []*SMTEnode, []SMTLiteral) {
	t.Helper()
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	_, constants := smtUFConstants(t, m, prefix, count)
	nodes := make([]*SMTEnode, count)
	for i, constant := range constants {
		node, err := s.MkEnode(constant, false, false, true)
		astTestMust(t, err)
		nodes[i] = node
	}
	result := make([]SMTLiteral, literals)
	for i := range result {
		atom := smtRelevancyBool(t, m, prefix+"-just-"+string(rune('a'+i)))
		variable, err := s.MkBoolVar(atom)
		astTestMust(t, err)
		result[i] = NewSMTLiteral(variable, false)
	}
	return m, s, nodes, result
}

func smtDiseqReason(t *testing.T, m *ast.ASTManager, s *SMTInternalizerState, first, second *SMTEnode) SMTLiteral {
	t.Helper()
	equality, err := m.MkEq(first.owner, second.owner)
	astTestMust(t, err)
	variable, err := s.MkBoolVar(equality)
	astTestMust(t, err)
	return NewSMTLiteral(variable, true)
}

func smtConflictAntecedents(t *testing.T, s *SMTInternalizerState) []SMTLiteral {
	t.Helper()
	conflict, found := s.Conflict()
	if !found {
		t.Fatal("expected a conflict")
	}
	object, ok := conflict.Justification()
	if !ok || object == nil {
		t.Fatalf("conflict has no justification object: %s", conflict.Kind())
	}
	if _, isDiseq := object.(*SMTDisequalityConflictJustification); !isDiseq {
		t.Fatalf("conflict justification %T", object)
	}
	antecedents, err := NewSMTAntecedentExpander().ExpandJustification(object)
	astTestMust(t, err)
	return antecedents
}

func requireAntecedents(t *testing.T, got []SMTLiteral, want ...SMTLiteral) {
	t.Helper()
	for _, literal := range want {
		if !slices.Contains(got, literal) {
			t.Fatalf("antecedents %v omit %d", got, literal)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("antecedents %v, want exactly %v", got, want)
	}
}

// The merge that closes a disequality depends on the merges behind both sides.
func TestDisequalityConflictExplainsPendingEqualityPaths(t *testing.T) {
	m, s, nodes, literals := smtDiseqFixture(t, "diseq-paths", 4, 3)
	a, b, c, d := nodes[0], nodes[1], nodes[2], nodes[3]
	astTestMust(t, s.addEquality(a, b, NewSMTEqualityEquation(literals[0])))
	astTestMust(t, s.addEquality(c, d, NewSMTEqualityEquation(literals[1])))
	astTestMust(t, s.addDisequality(b, d, smtDiseqReason(t, m, s, b, d)))
	astTestMust(t, s.addEquality(a, c, NewSMTEqualityEquation(literals[2])))
	requireAntecedents(t, smtConflictAntecedents(t, s), literals...)
}

// Merges made after the disequality move its terms away from the conflict's
// roots, so the disequality atom's own paths must also be explained.
func TestDisequalityConflictExplainsLaterMergesOfAtomTerms(t *testing.T) {
	m, s, nodes, literals := smtDiseqFixture(t, "diseq-atom", 3, 2)
	a, b, c := nodes[0], nodes[1], nodes[2]
	astTestMust(t, s.addDisequality(a, b, smtDiseqReason(t, m, s, a, b)))
	astTestMust(t, s.addEquality(a, c, NewSMTEqualityEquation(literals[0])))
	if s.inconsistent {
		t.Fatal("merge with an unrelated term must not conflict")
	}
	astTestMust(t, s.addEquality(c, b, NewSMTEqualityEquation(literals[1])))
	requireAntecedents(t, smtConflictAntecedents(t, s), literals...)
}

// Both operand orders reach the same explanation.
func TestDisequalityConflictExplanationIsOrderIndependent(t *testing.T) {
	for _, swap := range []bool{false, true} {
		m, s, nodes, literals := smtDiseqFixture(t, "diseq-order", 3, 2)
		a, b, c := nodes[0], nodes[1], nodes[2]
		astTestMust(t, s.addEquality(a, c, NewSMTEqualityEquation(literals[0])))
		astTestMust(t, s.addDisequality(c, b, smtDiseqReason(t, m, s, c, b)))
		left, right := a, b
		if swap {
			left, right = b, a
		}
		astTestMust(t, s.addEquality(left, right, NewSMTEqualityEquation(literals[1])))
		requireAntecedents(t, smtConflictAntecedents(t, s), literals...)
	}
}
