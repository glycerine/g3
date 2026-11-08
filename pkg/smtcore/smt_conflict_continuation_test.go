package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTAssignmentRetainsFirstConflictDuringContinuation(t *testing.T) {
	_, s, _, _, xs := smtPBWatchFixture(t, []int64{1, 1, 1}, 2, false)
	for _, l := range xs[:2] {
		astTestMust(t, s.AssignLiteral(l, NewSMTBooleanAxiom()))
	}
	astTestMust(t, s.AssignLiteral(xs[0].Negated(), NewSMTBooleanAxiom()))
	first, ok := s.Conflict()
	if !ok || s.ConflictLiteral() != xs[0] {
		t.Fatal("missing initial assignment conflict")
	}
	count := s.statistics.NumConflicts
	astTestMust(t, s.AssignLiteral(xs[1].Negated(), NewSMTBooleanBinaryClause(xs[0])))
	got, ok := s.Conflict()
	if !ok || got != first || s.ConflictLiteral() != xs[0] || s.statistics.NumConflicts != count {
		t.Fatal("later conflict replaced the first source conflict", s.ConflictLiteral(), s.statistics.NumConflicts)
	}
	astTestMust(t, s.AssignLiteral(xs[2], NewSMTBooleanAxiom()))
	v, err := s.Assignment(xs[2])
	astTestMust(t, err)
	if v != lbool.True {
		t.Fatal("source continuation lost a new assignment")
	}
}

func TestSMTClausePublicationContinuesWithoutReplacingConflict(t *testing.T) {
	_, s, _, _, xs := smtPBWatchFixture(t, []int64{1, 1, 1}, 2, false)
	for _, l := range xs {
		astTestMust(t, s.AssignLiteral(l.Negated(), NewSMTBooleanAxiom()))
	}
	astTestMust(t, s.AssignLiteral(xs[0], NewSMTBooleanAxiom()))
	first, _ := s.Conflict()
	count := s.statistics.NumConflicts
	before := len(s.clauses)
	astTestMust(t, s.addClauseKindWithJustificationAndDeleteHandler(xs[1:], false, SMTClauseAuxLemma, ast.ASTHandle{}, nil, nil))
	astTestMust(t, s.addClauseKindWithJustificationAndDeleteHandler(nil, false, SMTClauseAuxLemma, ast.ASTHandle{}, nil, nil))
	got, _ := s.Conflict()
	if len(s.clauses) != before+1 || got != first || s.ConflictLiteral() != xs[0].Negated() || s.statistics.NumConflicts != count {
		t.Fatal("source clause tail or first conflict was lost", len(s.clauses)-before, s.ConflictLiteral(), s.statistics.NumConflicts)
	}
}

func TestSMTPBAssignmentContinuesVariableWatchesAfterLiteralConflict(t *testing.T) {
	m, s, pb, a, xs := smtPBWatchFixture(t, []int64{2, 2, 1}, 3, false)
	var args []ast.ASTHandle
	for _, l := range xs {
		e, err := s.BoolExpression(l.Var())
		astTestMust(t, err)
		args = append(args, e)
	}
	expression := smtPBResolveExpression(t, m, pb.FamilyID(), args, []int32{1, 1, 1}, 2)
	astTestMust(t, s.Internalize(expression, true))
	b, err := s.Literal(expression)
	astTestMust(t, err)
	pb.controls.ConflictFrequency = ^uint32(0)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
	astTestMust(t, pb.Assign(a.Var(), true))
	for _, l := range xs {
		astTestMust(t, s.AssignLiteral(l.Negated(), NewSMTBooleanAxiom()))
	}
	// Pending callbacks are deliberately explicit: after x the weighted
	// watch still has sum 3; y conflicts before its variable-watch tail.
	astTestMust(t, pb.Assign(xs[0].Var(), false))
	if s.Inconsistent() {
		t.Fatal("first callback should only remove a literal watch")
	}
	astTestMust(t, pb.Assign(xs[1].Var(), false))
	value, err := s.Assignment(b)
	astTestMust(t, err)
	other := pb.info(b.Var()).constraint
	if !s.Inconsistent() || value != lbool.False || other.fixed != 2 || other.maxSum.RatString() != "1" {
		t.Fatal("literal conflict skipped source variable-watch continuation", value, other.fixed, other.maxSum.RatString())
	}
	astTestMust(t, s.PopScopes(1))
	value, err = s.Assignment(b)
	astTestMust(t, err)
	if s.Inconsistent() || value != lbool.Undef || other.fixed != 0 || other.maxSum.RatString() != "3" {
		t.Fatal("continued variable-watch trail did not restore its parent", value, other.fixed, other.maxSum.RatString())
	}
}

func TestSMTBinaryClausePublicationAssignsDuringContinuation(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(map[bool]string{false: "consistent", true: "existing-conflict"}[conflicting], func(t *testing.T) {
			_, s, _, _, xs := smtPBWatchFixture(t, []int64{1, 1, 1}, 2, false)
			s.binaryClauseOptimization = true
			s.relevancy = nil
			astTestMust(t, s.AssignLiteral(xs[0], NewSMTBooleanAxiom()))
			if conflicting {
				astTestMust(t, s.AssignLiteral(xs[0].Negated(), NewSMTBooleanAxiom()))
			}
			first, _ := s.Conflict()
			count := s.statistics.NumConflicts
			astTestMust(t, s.addClauseKindWithJustificationAndDeleteHandler([]SMTLiteral{xs[1], xs[0].Negated()}, false, SMTClauseAuxLemma, ast.ASTHandle{}, nil, nil))
			value, err := s.Assignment(xs[1])
			astTestMust(t, err)
			if value != lbool.True || len(s.clauses) != 1 || !s.clauses[0].Binary {
				t.Fatal("source binary-clause publication lost its immediate assignment", value, len(s.clauses))
			}
			reason, ok := s.booleanData[xs[1].Var()].Justification().Literal()
			if !ok || reason != xs[0] {
				t.Fatal("wrong source binary antecedent", reason, ok)
			}
			got, _ := s.Conflict()
			if conflicting && (got != first || s.ConflictLiteral() != xs[0] || s.statistics.NumConflicts != count) {
				t.Fatal("binary publication replaced the first conflict")
			}
		})
	}
}

func TestSMTEqualityConflictRetainsEarlierBooleanConflict(t *testing.T) {
	m := astTestManager(t)
	_, values := smtUFConstants(t, m, "continued-equality", 2)
	equality, err := m.MkEq(values[0], values[1])
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	astTestMust(t, s.Internalize(equality, false))
	eq, err := s.Literal(equality)
	astTestMust(t, err)
	p := smtRelevancyBool(t, m, "earlier-boolean-conflict")
	astTestMust(t, s.Internalize(p, true))
	boolean, err := s.Literal(p)
	astTestMust(t, err)
	left, right := s.context.enodes.get(values[0]), s.context.enodes.get(values[1])
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(eq.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, s.addDisequality(left, right, eq.Negated()))
	astTestMust(t, s.AssignLiteral(boolean, NewSMTBooleanAxiom()))
	astTestMust(t, s.AssignLiteral(boolean.Negated(), NewSMTBooleanAxiom()))
	first, ok := s.Conflict()
	if !ok {
		t.Fatal("missing initial Boolean conflict")
	}
	count := s.statistics.NumConflicts
	astTestMust(t, s.addEquality(left, right, NewSMTEqualityAxiom()))
	got, _ := s.Conflict()
	if got != first || s.ConflictLiteral() != boolean || s.statistics.NumConflicts != count {
		t.Fatal("later equality conflict replaced the first conflict", s.ConflictLiteral(), s.statistics.NumConflicts)
	}
	// add_eq merges the classes; the contradiction surfaces through the
	// false equality atom, after the first conflict is already set.
	astTestMust(t, s.PopScopes(1))
	if s.Inconsistent() || s.context.isDisequal(left, right) {
		t.Fatal("continuation scope did not restore the parent")
	}
}
