package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTInternalizerNewTrueEqualityPropagationAndPop(t *testing.T) {
	for _, gate := range []bool{false, true} {
		m := astTestManager(t)
		_, _, state := smtInternalizerFixture(t, m, nil)
		sort, values := smtUFConstants(t, m, "late-true-eq-", 2)
		function, err := m.MkFuncDecl(astTestSymbol(t, m, "late-true-eq-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		fa, err := m.MkApp(function, []ast.ASTHandle{values[0]})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(fa, false))
		fb, err := m.MkApp(function, []ast.ASTHandle{values[1]})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(fb, false))
		lateEquality, err := m.MkEq(fa, fb)
		astTestMust(t, err)
		astTestPin(t, m, lateEquality)
		astTestMust(t, state.PushScope())
		premise, err := m.MkEq(values[0], values[1])
		astTestMust(t, err)
		astTestMust(t, state.Internalize(premise, true))
		literal, err := state.Literal(premise)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if state.context.enodes.get(fa).Root() != state.context.enodes.get(fb).Root() {
			t.Fatal("fixture did not establish congruent application roots")
		}
		astTestMust(t, state.Internalize(lateEquality, gate))
		late, err := state.Literal(lateEquality)
		astTestMust(t, err)
		if value, err := state.Assignment(late); err != nil || value != lbool.True {
			t.Fatal("new equality over already merged operands was not propagated", gate, value, err)
		}
		justification, ok := state.booleanData[late.Var()].Justification().Justification()
		if _, equalityReason := justification.(*SMTEqualityPropagationJustification); !ok || !equalityReason {
			t.Fatal("new true equality lacks the native equality-propagation reason")
		}
		astTestMust(t, state.PopScopes(1))
		astTestMust(t, state.Internalize(lateEquality, gate))
		late, err = state.Literal(lateEquality)
		astTestMust(t, err)
		if value, err := state.Assignment(late); err != nil || value != lbool.Undef {
			t.Fatal("late true-equality assignment survived its congruence scope", value, err)
		}
	}
}

func TestSMTInternalizerNewTrueEqualityConflictsWithAssignedFalse(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	_, values := smtUFConstants(t, m, "late-conflicting-eq-", 2)
	for _, value := range values {
		astTestMust(t, state.Internalize(value, false))
	}
	astTestMust(t, state.addEquality(state.context.enodes.get(values[0]), state.context.enodes.get(values[1]), NewSMTEqualityAxiom()))
	_, err := state.PropagateCore()
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	equality, err := m.MkEq(values[0], values[1])
	astTestMust(t, err)
	astTestPin(t, m, equality)
	variable, err := state.MkBoolVar(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(variable, true), NewSMTBooleanAxiom()))
	astTestMust(t, state.ensureBooleanEnode(equality, variable, false, true, true))
	if !state.Inconsistent() {
		t.Fatal("a newly attached true equality ignored its opposite assignment")
	}
	astTestMust(t, state.PopScopes(1))
	if state.Inconsistent() || state.context.enodes.get(equality) != nil {
		t.Fatal("late-equality conflict or enode survived pop")
	}
}
