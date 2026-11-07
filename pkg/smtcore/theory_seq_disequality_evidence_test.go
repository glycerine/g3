package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSequenceUnitDisequalityGuardsAndScopedQueue(t *testing.T) {
	for _, mode := range []string{"propagate", "satisfied", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			falseGuard := mode == "satisfied"
			m := astTestManager(t)
			family, err := m.SequenceFamilyID()
			astTestMust(t, err)
			theory, err := NewSMTSequenceTheory(family)
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			astTestMust(t, state.RegisterTheory(theory))
			sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "disequality-elements"), nil)
			astTestMust(t, err)
			var elements, units []ast.ASTHandle
			for i := 0; i < 4; i++ {
				element, err := m.MkConst(astTestSymbol(t, m, fmt.Sprint("disequality-element-", i)), sort)
				astTestMust(t, err)
				unit, err := m.MkSequenceUnit(element)
				astTestMust(t, err)
				elements = append(elements, element)
				units = append(units, unit)
			}
			left, err := m.MkSequenceConcat(units[0], units[1])
			astTestMust(t, err)
			right, err := m.MkSequenceConcat(units[2], units[3])
			astTestMust(t, err)
			equality, err := m.MkEq(left, right)
			astTestMust(t, err)
			negative, err := m.MkNot(equality)
			astTestMust(t, err)
			astTestMust(t, state.Assert(negative, ast.ASTHandle{}, 0))
			_, err = state.PropagateCore()
			astTestMust(t, err)
			wordLiteral, err := state.Literal(equality)
			astTestMust(t, err)
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, state.PushScope())
				progress, err := theory.checkReducedDisequations()
				astTestMust(t, err)
				_, _, ls, rs, guards, _, err := theory.disequations.At(0)
				astTestMust(t, err)
				if !progress || len(ls) != 0 || len(rs) != 0 || len(guards) != 2 {
					t.Fatal("unit disequality did not become two element guards", progress, ls, rs, guards)
				}
				for i, pair := range [][2]int{{1, 3}, {0, 2}} {
					want, err := m.MkEq(elements[pair[0]], elements[pair[1]])
					astTestMust(t, err)
					got, err := state.BoolExpression(guards[i].Var())
					astTestMust(t, err)
					if got != want || !state.isRelevant(got) {
						t.Fatal("element guards lost source back-to-front order or relevance", got, want)
					}
				}
				assigned := guards[0]
				if falseGuard {
					assigned = assigned.Negated()
				}
				astTestMust(t, state.AssignLiteral(assigned, NewSMTBooleanAxiom()))
				if mode == "conflict" {
					astTestMust(t, state.AssignLiteral(guards[1], NewSMTBooleanAxiom()))
				}
				progress, err = theory.checkReducedDisequations()
				astTestMust(t, err)
				if progress == falseGuard || theory.disequations.Len() != 0 {
					t.Fatal("guard resolution did not remove its source queue slot", progress, theory.disequations.Len())
				}
				value, err := state.Assignment(guards[1])
				astTestMust(t, err)
				if mode == "conflict" {
					conflict, found := state.Conflict()
					owner, _ := conflict.Justification()
					ext, ok := owner.(*SMTTheoryConflictJustification)
					if !found || !ok || len(ext.Literals) != 3 {
						t.Fatal("all-true element guards did not produce an explained conflict", owner)
					}
				} else if falseGuard {
					if value != lbool.Undef {
						t.Fatal("satisfied disequality propagated an unnecessary element guard")
					}
				} else {
					if value != lbool.False {
						t.Fatal("last undefined element guard was not propagated false", value)
					}
					data, err := state.BooleanData(guards[1].Var())
					astTestMust(t, err)
					owner, _ := data.Justification().Justification()
					ext, ok := owner.(*SMTExtTheoryPropagationJustification)
					if !ok || len(ext.Literals) != 2 || ext.Literals[0] != wordLiteral.Negated() || ext.Literals[1] != guards[0] {
						t.Fatal("element propagation lost word disequality or true guard", owner)
					}
				}
				astTestMust(t, state.PopScopes(1))
				_, _, ls, rs, guards, _, err = theory.disequations.At(0)
				astTestMust(t, err)
				if len(ls) != 1 || len(rs) != 1 || len(guards) != 0 {
					t.Fatal("source word disequation did not restore after scoped reduction")
				}
			}
		})
	}
}

func TestSMTSequenceResidualDisequalityRetainsCancellationEvidence(t *testing.T) {
	m := astTestManager(t)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	text, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(text)
	astTestMust(t, err)
	var words []ast.ASTHandle
	for _, name := range []string{"premise-left", "premise-right", "residual-left", "residual-right"} {
		word, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		words = append(words, word)
		astTestMust(t, state.Internalize(word, false))
	}
	left, err := m.MkSequenceConcat(words[0], words[2])
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(words[1], words[3])
	astTestMust(t, err)
	original, err := m.MkEq(left, right)
	astTestMust(t, err)
	negative, err := m.MkNot(original)
	astTestMust(t, err)
	residual, err := m.MkEq(words[2], words[3])
	astTestMust(t, err)
	astTestMust(t, state.Internalize(original, true))
	astTestMust(t, state.Internalize(residual, true))
	originalLiteral, err := state.Literal(original)
	astTestMust(t, err)
	residualLiteral, err := state.Literal(residual)
	astTestMust(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, state.PushScope())
		astTestMust(t, state.Assert(negative, ast.ASTHandle{}, 0))
		astTestMust(t, state.addEquality(state.context.enodes.get(words[0]), state.context.enodes.get(words[1]), NewSMTEqualityAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		progress, err := theory.checkReducedDisequations()
		astTestMust(t, err)
		value, err := state.Assignment(residualLiteral)
		astTestMust(t, err)
		if !progress || value != lbool.False {
			t.Fatal("residual disequality was not propagated with its cancellation evidence", progress, value)
		}
		data, err := state.BooleanData(residualLiteral.Var())
		astTestMust(t, err)
		owner, ok := data.Justification().Justification()
		if !ok {
			t.Fatal("residual disequality has no theory explanation")
		}
		extended, ok := owner.(*SMTExtTheoryPropagationJustification)
		if !ok || len(extended.Literals) != 1 || extended.Literals[0] != originalLiteral.Negated() || len(extended.Equalities) != 1 {
			t.Fatal("residual disequality lost literal or equality premises", owner)
		}
		pair := extended.Equalities[0]
		if pair.First.root != pair.Second.root || pair.First.owner != words[0] && pair.First.owner != words[1] || pair.Second.owner != words[0] && pair.Second.owner != words[1] {
			t.Fatal("residual disequality uses unrelated cancellation evidence")
		}
		if progress, err := theory.checkReducedDisequations(); err != nil || progress {
			t.Fatal("already propagated residual reported progress", progress, err)
		}
		astTestMust(t, state.PopScopes(1))
		if value, err := state.Assignment(residualLiteral); err != nil || value != lbool.Undef || state.context.enodes.get(words[0]).root == state.context.enodes.get(words[1]).root {
			t.Fatal("scoped residual propagation or cancellation equality survived pop", value, err)
		}
	}
}
