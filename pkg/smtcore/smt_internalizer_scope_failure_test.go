package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTInternalizerPopResumesNativeRootRetirement(t *testing.T) {
	for _, owner := range []string{"enode", "boolean", "clause"} {
		for _, closeOnly := range []bool{false, true} {
			t.Run(owner+map[bool]string{false: "/retry", true: "/close"}[closeOnly], func(t *testing.T) {
				var state *SMTInternalizerState
				var failure error
				m, _, _, trigger := smtQuantifierRefusalFixture(t, func() {
					if closeOnly {
						failure = state.Close()
					} else {
						failure = state.PopScopes(1)
					}
				})
				_, _, state = smtInternalizerFixture(t, m, nil)
				smtTestUserPush(t, state)
				switch owner {
				case "enode":
					_, values := smtUFConstants(t, m, "pop-native-enode", 1)
					astTestMust(t, state.Internalize(values[0], false))
				case "boolean":
					_, err := state.MkBoolVar(smtRelevancyBool(t, m, "pop-native-boolean"))
					astTestMust(t, err)
				case "clause":
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					var expressions []ast.ASTHandle
					var literals []SMTLiteral
					for _, name := range []string{"pop-native-a", "pop-native-b", "pop-native-c"} {
						expression := smtRelevancyBool(t, m, name)
						variable, err := state.MkBoolVar(expression)
						astTestMust(t, err)
						expressions = append(expressions, expression)
						literals = append(literals, NewSMTLiteral(variable, false))
					}
					formula, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpOr, nil, expressions)
					astTestMust(t, err)
					proof, err := m.MkProofAsserted(formula)
					astTestMust(t, err)
					astTestMust(t, state.addLearnedClauseWithProof(literals, proof, false))
				}
				trigger()
				if !errors.Is(failure, ast.ErrASTPending) || state.scopeLevel != 1 || (!closeOnly && state.pendingPop == nil) || (closeOnly && !state.closing) {
					t.Fatal("native refusal lost its pending scope", failure, state.pendingPop, state.scopeLevel)
				}
				switch owner {
				case "enode":
					if state.retiringEnode == nil || state.retiringEnodeRoot.IsNull() {
						t.Fatal("enode retirement lost its live owner")
					}
				case "boolean":
					// Close retires the root scope, where context::init's
					// true/false enodes precede the Boolean variable's turn.
					if closeOnly && state.retiringEnode != nil && !state.retiringEnodeRoot.IsNull() {
						break
					}
					if !state.retiringBoolean || state.retiringBooleanRoot.IsNull() {
						t.Fatal("Boolean retirement lost its live owner")
					}
				case "clause":
					if len(state.clauses) != 1 || state.clauses[0].closeWatches != 2 || state.clauses[0].clause == nil {
						t.Fatal("clause retirement lost its detached owner", state.clauses)
					}
				}
				if err := state.PushScope(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("pending pop accepted another scope", err)
				}
				if err := state.PopScopes(2); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("pending pop accepted a different count", err)
				}
				if closeOnly {
					if err := state.PopScopes(1); !errors.Is(err, ast.ErrASTPending) {
						t.Fatal("destruction accepted scope restoration", err)
					}
					astTestMust(t, state.Close())
					return
				}
				astTestMust(t, state.PopScopes(1))
				if state.scopeLevel != 0 || state.pendingPop != nil || len(state.booleanStack) != 0 || len(state.enodeStack) != smtInitEnodes || len(state.clauses) != 0 {
					t.Fatal("pop retry left owned scope state")
				}
				smtTestUserPush(t, state)
				astTestMust(t, state.PopScopes(1))
			})
		}
	}
}

func TestSMTInternalizerPopRestoresCongruenceDuringRefusal(t *testing.T) {
	var state *SMTInternalizerState
	var failure error
	m, _, _, trigger := smtQuantifierRefusalFixture(t, func() { failure = state.PopScopes(1) })
	_, _, state = smtInternalizerFixture(t, m, nil)
	sort, values := smtUFConstants(t, m, "pop-congruence", 2)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "pop-congruence-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	var applications []ast.ASTHandle
	for _, value := range values {
		application, err := m.MkApp(function, []ast.ASTHandle{value})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(application, false))
		applications = append(applications, application)
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(state.context.enodes.get(values[0]), state.context.enodes.get(values[1]), NewSMTEqualityAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	trigger()
	t.Log("first pop:", failure)
	// A refused owner leaves the pop pending; the retry must finish it once.
	if failure != nil {
		astTestMust(t, state.PopScopes(1))
	}
	if len(state.equalityTrail) != 0 || state.scopeLevel != 0 || state.context.enodes.get(applications[0]).Root() == state.context.enodes.get(applications[1]).Root() {
		t.Fatal("pop did not restore independent application classes")
	}
	astTestMust(t, state.congruence.CheckInvariant())
}
