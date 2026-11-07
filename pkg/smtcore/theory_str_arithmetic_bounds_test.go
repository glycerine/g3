package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func stringAssertArithmeticBound(t *testing.T, m *ast.ASTManager, state *SMTInternalizerState, expression ast.ASTHandle, operation int32, numeral string, real bool) {
	t.Helper()
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	var number ast.ASTHandle
	if real {
		number, err = m.ArithmeticRealNumeralText(numeral)
	} else {
		number, err = m.ArithmeticIntegerNumeralText(numeral)
	}
	astTestMust(t, err)
	// theory_arith atoms are the rewriter's normal form: (> a k) is
	// (not (<= a k)) and (< a k) is (not (>= a k)).
	negated := false
	switch operation {
	case smtArithOpGT:
		operation, negated = smtArithOpLE, true
	case smtArithOpLT:
		operation, negated = smtArithOpGE, true
	}
	atom, err := m.MkFamilyApp(family, operation, nil, []ast.ASTHandle{expression, number})
	astTestMust(t, err)
	astTestPin(t, m, atom)
	astTestMust(t, state.Internalize(atom, true))
	astTestMust(t, state.markRelevant(atom))
	literal, err := state.Literal(atom)
	astTestMust(t, err)
	if negated {
		literal = literal.Negated()
	}
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if state.inconsistent {
		t.Fatal("bound fixture became inconsistent")
	}
}

func stringCheckArithmeticBound(t *testing.T, theory *SMTStringTheory, expression ast.ASTHandle, lower bool, want string) {
	t.Helper()
	value, known, err := theory.stringArithmeticBound(expression, lower)
	astTestMust(t, err)
	if want == "" {
		if known || value != nil {
			t.Fatal("unjustified or noninteger bound returned", value, known)
		}
		return
	}
	if !known || value.String() != want {
		t.Fatal("arithmetic bound changed", value, known, want)
	}
	// Returned integers must not alias the live tableau's bound storage.
	value.SetInt64(-999)
	again, known, err := theory.stringArithmeticBound(expression, lower)
	astTestMust(t, err)
	if !known || again.String() != want {
		t.Fatal("caller mutated stored arithmetic bound", again, known)
	}
}

func TestSMTStringArithmeticBoundsAssignmentAndScope(t *testing.T) {
	m := astTestManager(t)
	theory, state, arithmetic := stringMixedEqualityFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	word, err := m.MkConst(astTestSymbol(t, m, "regex-bounded-word"), sort)
	astTestMust(t, err)
	astTestPin(t, m, word)
	length, err := m.MkSequenceLength(word)
	astTestMust(t, err)
	astTestPin(t, m, length)
	stringCheckArithmeticBound(t, theory, length, true, "")
	stringAssertArithmeticBound(t, m, state, length, smtArithOpGE, "2", false)
	// The length is attached to an arithmetic variable once an arithmetic
	// atom mentions it.
	variable := state.context.enodes.get(length).TheoryVar(arithmetic.FamilyID())
	arithmetic.value[variable].rational.SetInt64(6) // a candidate value, not a bound
	stringCheckArithmeticBound(t, theory, length, true, "2")
	stringCheckArithmeticBound(t, theory, length, false, "")
	stringAssertArithmeticBound(t, m, state, length, smtArithOpLE, "9", false)
	stringCheckArithmeticBound(t, theory, length, true, "2")
	stringCheckArithmeticBound(t, theory, length, false, "9")
	astTestMust(t, state.PushScope())
	stringAssertArithmeticBound(t, m, state, length, smtArithOpGE, "5", false)
	stringAssertArithmeticBound(t, m, state, length, smtArithOpLE, "7", false)
	stringCheckArithmeticBound(t, theory, length, true, "5")
	stringCheckArithmeticBound(t, theory, length, false, "7")
	astTestMust(t, state.PopScopes(1))
	stringCheckArithmeticBound(t, theory, length, true, "2")
	stringCheckArithmeticBound(t, theory, length, false, "9")
	// Neither bound makes an exact core-root value available.
	if _, known, err := theory.arithmeticValue(length); err != nil || known {
		t.Fatal("bounds became an equality premise", known, err)
	}
}

func TestSMTStringArithmeticBoundsMixedGateAndNonintegerRefusals(t *testing.T) {
	m := astTestManager(t)
	integerTheory, _ := stringTheoryFixture(t, m)
	stringCheckArithmeticBound(t, integerTheory, astTestString(t, m, "ab"), true, "")
	m = astTestManager(t)
	theory, state, arithmetic := stringMixedEqualityFixture(t, m)
	family := arithmetic.FamilyID()
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "regex-real-bound"), sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	stringAssertArithmeticBound(t, m, state, x, smtArithOpGT, "2", true)
	stringAssertArithmeticBound(t, m, state, x, smtArithOpLE, "7/2", true)
	stringCheckArithmeticBound(t, theory, x, true, "")  // Infinitesimal lower bound.
	stringCheckArithmeticBound(t, theory, x, false, "") // Nonintegral rational upper bound.
	astTestMust(t, state.PushScope())
	stringAssertArithmeticBound(t, m, state, x, smtArithOpGE, "3", true)
	stringCheckArithmeticBound(t, theory, x, true, "3") // Integral Real bounds are accepted.
	astTestMust(t, state.PopScopes(1))
	stringCheckArithmeticBound(t, theory, x, true, "")
}

func TestSMTStringArithmeticBoundsUnassignedAndWideEndpoints(t *testing.T) {
	m := astTestManager(t)
	theory, state, arithmetic := stringMixedEqualityFixture(t, m)
	family := arithmetic.FamilyID()
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "regex-wide-bound"), sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	number, err := m.ArithmeticIntegerNumeralText("-18446744073709551617")
	astTestMust(t, err)
	atom, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, number})
	astTestMust(t, err)
	astTestPin(t, m, atom)
	astTestMust(t, state.Internalize(atom, true))
	stringCheckArithmeticBound(t, theory, x, true, "")
	stringAssertArithmeticBound(t, m, state, x, smtArithOpGE, "-18446744073709551617", false)
	stringAssertArithmeticBound(t, m, state, x, smtArithOpLE, "18446744073709551617", false)
	stringCheckArithmeticBound(t, theory, x, true, "-18446744073709551617")
	stringCheckArithmeticBound(t, theory, x, false, "18446744073709551617")
}
