package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringConversionCoreEquality(t *testing.T, state *SMTInternalizerState, left, right ast.ASTHandle) {
	t.Helper()
	equality, err := state.manager.MkEq(left, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
}

func TestSMTStringToIntAgreementZeroSplitIsUnasserted(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "stoi-split-word")
	expression, err := m.MkStringToInt(word)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	before := len(theory.refs)
	for repeat := 0; repeat < 2; repeat++ {
		added, err := theory.finalCheckStringToInt(expression)
		astTestMust(t, err)
		if !added || len(theory.refs) != before || len(theory.conversionAxioms) != 0 {
			t.Fatal("unknown arithmetic conversion must request continuation without asserting zero")
		}
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	isZero, err := m.MkEq(expression, zero)
	astTestMust(t, err)
	literal, err := state.Literal(isZero)
	astTestMust(t, err)
	value, err := state.Assignment(literal)
	astTestMust(t, err)
	if value != lbool.Undef || state.context.enodes.get(isZero) == nil {
		t.Fatal("source zero split was asserted or omitted its enode", value)
	}
}

func TestSMTStringToIntAgreementUsesActualNumeralRoot(t *testing.T) {
	for _, number := range []string{"-2", "-1", "0", "42", "999999999999999999999999999999999999"} {
		t.Run(number, func(t *testing.T) {
			m, theory, state, rw := stringLibraryFixture(t)
			word := stringContainmentWord(t, theory, "stoi-forward-word")
			expression, err := m.MkStringToInt(word)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(expression, false))
			numeral, err := m.ArithmeticIntegerNumeralText(number)
			astTestMust(t, err)
			stringConversionCoreEquality(t, state, expression, numeral)
			value, known, err := theory.arithmeticValue(expression)
			astTestMust(t, err)
			if !known || value.String() != number {
				t.Fatal("fixture did not establish an actual numeral root")
			}
			before := len(theory.refs)
			added, err := theory.finalCheckStringToInt(expression)
			astTestMust(t, err)
			if number == "-1" {
				if added || len(theory.refs) != before || len(theory.conversionAxioms) != 0 {
					t.Fatal("minus-one result incorrectly forced an input string")
				}
				return
			}
			if !added || len(theory.conversionAxioms) != 1 {
				t.Fatal("numeric conversion root did not emit canonical string agreement")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			for _, text := range []string{number, "#wrong#"} {
				values := map[ast.ASTHandle]ast.ASTHandle{expression: numeral, word: astTestString(t, m, text)}
				if truth := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); truth != (text == number) {
					t.Fatal("actual numeral-root agreement lost its canonical string", text, number, truth)
				}
			}
			before = len(theory.refs)
			added, err = theory.finalCheckStringToInt(expression)
			astTestMust(t, err)
			if added || len(theory.refs) != before {
				t.Fatal("same forward agreement bypassed memoization")
			}
		})
	}
}

func TestSMTStringToIntAgreementParsesPrivateClassBytes(t *testing.T) {
	for _, test := range []struct{ text, number string }{
		{"", "-1"}, {"0", "0"}, {"0007", "7"}, {"42", "42"},
		{"999999999999999999999999999999999999", "999999999999999999999999999999999999"},
		{"-1", "-1"}, {"+2", "-1"}, {" 2", "-1"}, {"2.0", "-1"}, {"2\x00", "-1"}, {"\xff", "-1"},
	} {
		t.Run(test.text, func(t *testing.T) {
			m, theory, state, rw := stringLibraryFixture(t)
			word := stringContainmentWord(t, theory, "stoi-backward-word")
			expression, err := m.MkStringToInt(word)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(expression, false))
			constant := astTestString(t, m, test.text)
			astTestMust(t, state.Internalize(constant, false))
			// Exercise the source private class, independent of core congruence.
			theory.find.Merge(uint32(state.context.enodes.get(word).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(constant).TheoryVar(theory.FamilyID())))
			added, err := theory.finalCheckStringToInt(expression)
			astTestMust(t, err)
			if !added || len(theory.conversionAxioms) != 1 {
				t.Fatal("private concrete class did not emit parsed conversion agreement")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			for _, number := range []string{test.number, "-999"} {
				value, err := m.ArithmeticIntegerNumeralText(number)
				astTestMust(t, err)
				values := map[ast.ASTHandle]ast.ASTHandle{word: constant, expression: value}
				if truth := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); truth != (number == test.number) {
					t.Fatal("private string byte conversion lost its parsed result", test.text, number, test.number, truth)
				}
			}
			before := len(theory.refs)
			added, err = theory.finalCheckStringToInt(expression)
			astTestMust(t, err)
			// Unknown arithmetic always requests continuation for the zero split,
			// while the already-published string agreement stays memoized.
			if !added || len(theory.refs) != before {
				t.Fatal("repeated unknown-arithmetic agreement changed source continuation/memoization")
			}
		})
	}
}

func TestSMTIntToStringAgreementParsesOrRefusesClassValue(t *testing.T) {
	for _, test := range []struct{ text, number string }{{"", ""}, {"0", "0"}, {"0007", "7"}, {"42", "42"}, {"999999999999999999999999999999999999", "999999999999999999999999999999999999"}, {"-1", ""}, {"1\x00", ""}, {"\xff", ""}} {
		t.Run(test.text, func(t *testing.T) {
			m, theory, state, rw := stringLibraryFixture(t)
			integer := stringLibraryInteger(t, m, state, "itos-agreement-integer")
			expression, err := m.MkIntToString(integer)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(expression, false))
			constant := astTestString(t, m, test.text)
			astTestMust(t, state.Internalize(constant, false))
			theory.find.Merge(uint32(state.context.enodes.get(expression).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(constant).TheoryVar(theory.FamilyID())))
			before := len(theory.refs)
			added, err := theory.finalCheckIntToString(expression)
			astTestMust(t, err)
			if test.text == "" {
				if added || len(theory.refs) != before {
					t.Fatal("empty conversion result repeated its initial axiom")
				}
				return
			}
			if !added {
				t.Fatal("nonempty conversion result skipped its agreement/refusal")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			if test.number != "" {
				for _, number := range []string{test.number, "-999"} {
					value, err := m.ArithmeticIntegerNumeralText(number)
					astTestMust(t, err)
					values := map[ast.ASTHandle]ast.ASTHandle{integer: value, expression: constant}
					if truth := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); truth != (number == test.number) {
						t.Fatal("int-to-string agreement lost parsed integer", test.text, number, truth)
					}
				}
			} else if stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, map[ast.ASTHandle]ast.ASTHandle{expression: constant}) {
				t.Fatal("nondecimal int-to-string class value was not refused")
			}
			before = len(theory.refs)
			added, err = theory.finalCheckIntToString(expression)
			astTestMust(t, err)
			if test.number == "" {
				if !added || len(theory.refs) != before+1 || len(theory.conversionAxioms) != 0 {
					t.Fatal("native invalid-value conflict must be republished without memoization")
				}
			} else if added || len(theory.refs) != before || len(theory.conversionAxioms) != 1 {
				t.Fatal("valid int-to-string agreement bypassed memoization")
			}
		})
	}
}

func TestSMTConversionAgreementMemoScopeRollbackAndMissingClass(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	integer := stringLibraryInteger(t, m, state, "itos-scoped-integer")
	expression, err := m.MkIntToString(integer)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	if added, err := theory.finalCheckIntToString(expression); err == nil || added {
		t.Fatal("unassigned conversion class bypassed native final-stage precondition")
	}
	constant := astTestString(t, m, "42")
	astTestMust(t, state.Internalize(constant, false))
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, state.PushScope())
		theory.find.Merge(uint32(state.context.enodes.get(expression).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(constant).TheoryVar(theory.FamilyID())))
		added, err := theory.finalCheckIntToString(expression)
		astTestMust(t, err)
		if !added || len(theory.conversionAxioms) != 1 {
			t.Fatal("scoped conversion agreement was suppressed by a prior popped visit")
		}
		astTestMust(t, state.PopScopes(1))
		if len(theory.conversionAxioms) != 0 {
			t.Fatal("conversion agreement memo survived its source insertion trail")
		}
	}
}

func TestSMTConversionAgreementMemoKeepsOuterScopes(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	var expressions [3]ast.ASTHandle
	for i, name := range []string{"itos-outer", "itos-middle", "itos-inner"} {
		integer := stringLibraryInteger(t, m, state, name)
		var err error
		expressions[i], err = m.MkIntToString(integer)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(expressions[i], false))
	}
	constant := astTestString(t, m, "42")
	astTestMust(t, state.Internalize(constant, false))
	for i, expression := range expressions {
		if i > 0 {
			astTestMust(t, state.PushScope())
		}
		theory.find.Merge(uint32(state.context.enodes.get(expression).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(constant).TheoryVar(theory.FamilyID())))
		added, err := theory.finalCheckIntToString(expression)
		astTestMust(t, err)
		if !added || len(theory.conversionAxioms) != i+1 {
			t.Fatal("missing nested conversion agreement memo", i)
		}
	}
	for remaining := 2; remaining > 0; remaining-- {
		astTestMust(t, state.PopScopes(1))
		if len(theory.conversionAxioms) != remaining {
			t.Fatal("pop removed an outer conversion memo or retained an inner one", remaining, len(theory.conversionAxioms))
		}
		for _, expression := range expressions[:remaining] {
			added, err := theory.finalCheckIntToString(expression)
			astTestMust(t, err)
			if added {
				t.Fatal("pop erased a still-live outer agreement memo")
			}
		}
	}
}
