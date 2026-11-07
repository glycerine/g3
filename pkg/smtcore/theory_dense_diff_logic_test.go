package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTDenseTheoryPropagationConflictAndScope(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTDenseDiffTheory(family)
			astTestMust(t, err)
			astTestMust(t, theory.ConfigureArithmetic(SMTArithState{Reflect: true}))
			astTestMust(t, state.RegisterTheory(theory))
			kind := int32(0)
			if integer {
				kind = 1
			}
			sort, err := m.MkFamilySort(family, kind, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "dense-context-x"), sort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "dense-context-y"), sort)
			astTestMust(t, err)
			bound := func(left, right ast.ASTHandle, n int64) ast.ASTHandle {
				if integer {
					fixture := smtIntegerDiffFixture{manager: m, family: family, sort: sort, x: x, y: y}
					return fixture.bound(t, smtArithOpLE, left, right, n)
				}
				fixture := smtRealDiffFixture{manager: m, family: family, sort: sort, x: x, y: y}
				return fixture.bound(t, smtArithOpLE, left, right, n, 1)
			}
			strong, weak := bound(x, y, 0), bound(x, y, 2)
			astTestMust(t, state.Internalize(strong, true))
			astTestMust(t, state.Internalize(weak, true))
			strongLiteral, err := state.Literal(strong)
			astTestMust(t, err)
			weakLiteral, err := state.Literal(weak)
			astTestMust(t, err)
			astTestMust(t, state.PushScope())
			astTestMust(t, state.AssignLiteral(strongLiteral, NewSMTBooleanAxiom()))
			astTestMust(t, state.PropagateTheoryAssignments())
			assignment, err := state.Assignment(weakLiteral)
			astTestMust(t, err)
			if assignment != lbool.True {
				t.Fatal("dense cell did not propagate weaker bound", assignment)
			}
			data, err := state.BooleanData(weakLiteral.Var())
			astTestMust(t, err)
			proof, ok := data.Justification().Justification()
			if !ok || proof.FromTheory() != family {
				t.Fatal("dense propagation lost theory provenance")
			}
			conflicting := bound(y, x, -1)
			astTestMust(t, state.Internalize(conflicting, true))
			conflictingLiteral, err := state.Literal(conflicting)
			astTestMust(t, err)
			astTestMust(t, state.AssignLiteral(conflictingLiteral, NewSMTBooleanAxiom()))
			astTestMust(t, state.PropagateTheoryAssignments())
			if !state.inconsistent {
				t.Fatal("dense reverse path did not conflict")
			}
			astTestMust(t, state.PopScopes(1))
			assignment, err = state.Assignment(weakLiteral)
			astTestMust(t, err)
			if state.inconsistent || assignment != lbool.Undef || len(theory.atoms) != 2 || len(theory.matrix.edges) != 1 {
				t.Fatal("dense child state survived pop")
			}
			astTestMust(t, state.AssignLiteral(strongLiteral.Negated(), NewSMTBooleanAxiom()))
			astTestMust(t, state.PropagateTheoryAssignments())
			if !integer {
				fixture := smtRealDiffFixture{manager: m, family: family, sort: sort, x: x, y: y}
				upper := fixture.bound(t, smtArithOpLE, x, y, 1, 3)
				astTestMust(t, state.Internalize(upper, true))
				literal, err := state.Literal(upper)
				astTestMust(t, err)
				astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
				astTestMust(t, state.PropagateTheoryAssignments())
			}
			status, err := theory.FinalCheck()
			astTestMust(t, err)
			if status != SMTFinalCheckDone {
				t.Fatal("dense complement failed", status)
			}
			generator, err := NewSMTModelGenerator(m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, generator.Close()) })
			astTestMust(t, generator.SetContext(state.context))
			astTestMust(t, generator.InitializeModel(cfg))
			astTestMust(t, state.context.SetRelevant(x, true))
			astTestMust(t, state.context.SetRelevant(y, true))
			astTestMust(t, generator.BuildStateValues(state))
			xValue, err := generator.Value(state.context.enodes.get(x))
			astTestMust(t, err)
			yValue, err := generator.Value(state.context.enodes.get(y))
			astTestMust(t, err)
			xText, _, _, err := m.ArithmeticNumeralText(xValue)
			astTestMust(t, err)
			yText, _, _, err := m.ArithmeticNumeralText(yValue)
			astTestMust(t, err)
			xRat, xOK := new(big.Rat).SetString(xText)
			yRat, yOK := new(big.Rat).SetString(yText)
			if !xOK || !yOK {
				t.Fatal("dense model did not publish numeral values", xText, yText)
			}
			gap := new(big.Rat).Sub(xRat, yRat)
			if gap.Sign() <= 0 || !integer && gap.Cmp(big.NewRat(1, 3)) > 0 {
				t.Fatal("published dense model violates strict complement or rational upper bound", xText, yText)
			}
			xv := state.context.enodes.get(x).TheoryVar(family)
			yv := state.context.enodes.get(y).TheoryVar(family)
			var difference smtInfRational
			difference.Sub(&theory.values[xv], &theory.values[yv])
			if difference.Sign() <= 0 {
				t.Fatal("complement did not yield x > y")
			}
		})
	}
}
