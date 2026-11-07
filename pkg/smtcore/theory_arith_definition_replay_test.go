package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTArithmeticDefinitionsPropagateWithoutBooleanAtoms(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	theory, err := NewSMTMixedLinearArithmeticTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	for _, text := range []string{"7", "-3", "5/2"} {
		for attempt := 0; attempt < 2; attempt++ {
			astTestMust(t, state.PushScope())
			want, ok := new(big.Rat).SetString(text)
			if !ok {
				t.Fatal("invalid test numeral")
			}
			number, err := m.ArithmeticNumeralRat(want, want.IsInt())
			astTestMust(t, err)
			astTestMust(t, state.Internalize(number, false))
			consistent, err := state.PropagateCore()
			astTestMust(t, err)
			column := theory.simplexVariables[state.context.enodes.get(number).TheoryVar(family)]
			if !consistent || theory.arithmeticColumnValue(column).Cmp(want) != 0 {
				t.Fatal("standalone numeral lost its fixed value", text, theory.arithmeticColumnValue(column))
			}
			// A later definition must invalidate the replay even though no
			// arithmetic predicate was created or assigned in either step.
			sum, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{number, number})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(sum, false))
			consistent, err = state.PropagateCore()
			astTestMust(t, err)
			column = theory.simplexVariables[state.context.enodes.get(sum).TheoryVar(family)]
			want.Mul(want, big.NewRat(2, 1))
			if !consistent || theory.arithmeticColumnValue(column).Cmp(want) != 0 || len(theory.atoms) != 0 || theory.CanPropagate() {
				t.Fatal("new arithmetic definition was not published without a Boolean atom", text, theory.arithmeticColumnValue(column))
			}
			astTestMust(t, state.PopScopes(1))
			if theory.NumVariables() != 0 || len(theory.definitions) != 0 || theory.CanPropagate() {
				t.Fatal("scoped arithmetic definitions survived pop")
			}
		}
	}
}
