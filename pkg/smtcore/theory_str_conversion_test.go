package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringToIntInitialBoundsZeroAndDecimalLanguage(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "stoi-word")
	expression, err := m.MkStringToInt(word)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	before := len(theory.refs)
	astTestMust(t, theory.instantiateStringToInt(state.context.enodes.get(expression)))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	var axioms []ast.ASTHandle
	for _, ref := range theory.refs[before:] {
		value, err := m.RefGet(ref)
		astTestMust(t, err)
		sort, err := m.GetSort(value)
		astTestMust(t, err)
		if sort == boolean {
			axioms = append(axioms, value)
		}
	}
	if len(axioms) != 3 || len(theory.libraryTodo) != 1 {
		t.Fatal("string-to-int initial cases or immediate regex setup missing", len(axioms), len(theory.libraryTodo))
	}
	member := theory.libraryTodo[0].Owner()
	op, own, err := theory.operation(member)
	astTestMust(t, err)
	if !own || op != 12 {
		t.Fatal("positive conversion did not internalize and enqueue regex membership")
	}
	for _, text := range []string{"", "0", "00", "01", "1", "9", "10", "999999999999999999999999999999999999", "x", "-1", "+1", "1.0", " 1", "1\n", "\x00", "\xff", "123\x004"} {
		canonicalPositive := len(text) > 0 && text[0] >= '1' && text[0] <= '9'
		for i := 1; i < len(text); i++ {
			canonicalPositive = canonicalPositive && text[i] >= '0' && text[i] <= '9'
		}
		for _, number := range []string{"-2", "-1", "0", "1", "2", "9", "999999999999999999999999999999999999"} {
			numeral, err := m.ArithmeticIntegerNumeralText(number)
			astTestMust(t, err)
			value, ok := new(big.Int).SetString(number, 10)
			if !ok {
				t.Fatal("invalid independent test numeral")
			}
			want := value.Cmp(big.NewInt(-1)) >= 0 && (value.Sign() == 0) == (text == "0") && (value.Sign() <= 0 || canonicalPositive)
			values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), expression: numeral}
			if truth := stringLibraryAxiomsTruth(t, m, rw, axioms, values); truth != want {
				// These are syntax/zero constraints, not final decimal value agreement.
				t.Fatal("initial string-to-int bound/zero/language formula changed", text, number, want, truth)
			}
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateStringToInt(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh || len(theory.libraryTodo) != 1 {
		t.Fatal("duplicate string-to-int repeated constraints or regex work")
	}
}

func TestSMTIntToStringInitialNegativeEmptyEquivalence(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	integer := stringLibraryInteger(t, m, state, "itos-integer")
	expression, err := m.MkIntToString(integer)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.instantiateIntToString(state.context.enodes.get(expression)))
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	for _, number := range []string{"-999999999999999999999999999999999999", "-1", "0", "1", "42", "999999999999999999999999999999999999"} {
		for _, text := range []string{"", "0", "42", "x", "\x00", "\xff"} {
			numeral, err := m.ArithmeticIntegerNumeralText(number)
			astTestMust(t, err)
			values := map[ast.ASTHandle]ast.ASTHandle{integer: numeral, expression: astTestString(t, m, text)}
			want := (number[0] == '-') == (text == "")
			if truth := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); truth != want {
				t.Fatal("initial int-to-string negative/empty equivalence changed", number, text, want, truth)
			}
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateIntToString(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("duplicate int-to-string repeated its initial constraint")
	}
}

func TestSMTStringConversionLibraryQueueAndRegexRegistration(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "conversion-queue-word")
	integer := stringLibraryInteger(t, m, state, "conversion-queue-integer")
	toInt, err := m.MkStringToInt(word)
	astTestMust(t, err)
	toString, err := m.MkIntToString(integer)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{toInt, toString} {
		astTestMust(t, state.Internalize(expression, false))
		astTestMust(t, theory.setUpAxioms(expression))
	}
	astTestMust(t, theory.Propagate())
	if theory.CanPropagate() || len(theory.conversions) != 2 || len(theory.regexMemberships[word]) != 1 || len(theory.regexTerms) != 1 {
		t.Fatal("conversion worklist did not register and drain its actual regex child", theory.CanPropagate(), len(theory.conversions), len(theory.regexMemberships[word]), len(theory.regexTerms))
	}
	for _, expression := range []ast.ASTHandle{toInt, toString} {
		if _, found := theory.axiomatized[expression]; !found {
			t.Fatal("conversion worklist skipped an initial-axiom producer")
		}
	}
	for expression := range theory.regexTerms {
		if _, found := theory.axiomatized[expression]; !found {
			t.Fatal("conversion regex child bypassed membership dispatch")
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.Propagate())
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("quiescent conversion worklist repeated initial axioms")
	}
}

func TestSMTStringConversionRegistryOwnsRepeatedSetupVisits(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "conversion-registry-word")
	integer := stringLibraryInteger(t, m, state, "conversion-registry-integer")
	toInt, err := m.MkStringToInt(word)
	astTestMust(t, err)
	toString, err := m.MkIntToString(integer)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{toString, toInt, toString, toInt} {
		astTestMust(t, state.Internalize(expression, false))
		astTestMust(t, theory.setUpAxioms(expression))
	}
	want := []ast.ASTHandle{toString, toInt, toString, toInt}
	if len(theory.conversions) != len(want) || len(theory.conversionRefs) != len(want) {
		t.Fatal("conversion registry lost repeated source visits")
	}
	roots := append([]ast.ASTRef(nil), theory.conversionRefs...)
	for i, root := range roots {
		actual, err := m.RefGet(root)
		astTestMust(t, err)
		if actual != want[i] || theory.conversions[i] != want[i] {
			t.Fatal("conversion registry changed insertion order or ownership", i)
		}
	}
	// No initial axioms have run: the registry itself must own every visit.
	astTestMust(t, theory.Close())
	for _, root := range roots {
		if _, err := m.RefGet(root); err == nil {
			t.Fatal("conversion registry retained an owned root after close")
		}
	}
	if len(theory.conversions) != 0 || len(theory.conversionRefs) != 0 {
		t.Fatal("closed conversion registry still exposes entries")
	}
}
