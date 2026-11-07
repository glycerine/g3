package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTStringConcatCanonicalizationCacheAndScope(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-concat-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "str-concat-y"), sort)
	astTestMust(t, err)
	a, b, empty := astTestString(t, m, "a"), astTestString(t, m, "b"), astTestString(t, m, "")
	xa, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	by, err := m.MkSequenceConcat(b, y)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{xa, by, empty} {
		astTestMust(t, state.Internalize(expression, false))
	}
	combined, err := theory.concatTerm(a, b)
	astTestMust(t, err)
	if text, constant, err := m.SequenceStringValue(combined); err != nil || !constant || text != "ab" {
		t.Fatal("known literal classes did not combine", text, err)
	}
	for _, pair := range [][2]ast.ASTHandle{{empty, x}, {x, empty}} {
		got, err := theory.concatTerm(pair[0], pair[1])
		astTestMust(t, err)
		if got != x {
			t.Fatal("known empty class did not disappear")
		}
	}
	for _, pair := range [][2]ast.ASTHandle{{a, by}, {xa, b}, {xa, by}} {
		got, err := theory.concatTerm(pair[0], pair[1])
		astTestMust(t, err)
		parts, err := m.SequenceFlattenConcat(got)
		astTestMust(t, err)
		merged := false
		for _, part := range parts {
			text, literal, err := m.SequenceStringValue(part)
			astTestMust(t, err)
			merged = merged || literal && text == "ab"
		}
		if !merged {
			t.Fatal("adjacent literal boundary was not merged")
		}
		before := len(theory.refs)
		again, err := theory.concatTerm(pair[0], pair[1])
		astTestMust(t, err)
		if again != got || len(theory.refs) != before {
			t.Fatal("concat cache repeated its AST/length-axiom publication")
		}
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.NewEquality(state.context.enodes.get(x).TheoryVar(theory.FamilyID()), state.context.enodes.get(a).TheoryVar(theory.FamilyID())))
	got, err := theory.concatTerm(x, b)
	astTestMust(t, err)
	if text, _, err := m.SequenceStringValue(got); err != nil || text != "ab" {
		t.Fatal("concat did not use its current private classes", text, err)
	}
	astTestMust(t, state.PopScopes(1))
	got, err = theory.concatTerm(x, b)
	astTestMust(t, err)
	if _, constant, err := m.SequenceStringValue(got); err != nil || constant {
		t.Fatal("concat reused a popped private-class value", err)
	}
}

func TestSMTStringContainsConstantFastPath(t *testing.T) {
	for _, test := range []struct {
		haystack, needle string
		want             bool
	}{{"abc", "b", true}, {"abc", "z", false}, {"", "", true}, {string([]byte{0, 255}), string([]byte{255}), true}} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		relation, err := m.MkSequenceContains(astTestString(t, m, test.haystack), astTestString(t, m, test.needle))
		astTestMust(t, err)
		astTestMust(t, state.Internalize(relation, true))
		astTestMust(t, theory.instantiateContains(state.context.enodes.get(relation)))
		literal, err := state.Literal(relation)
		astTestMust(t, err)
		value, err := state.Assignment(literal)
		astTestMust(t, err)
		if (value == lbool.True) != test.want || value == lbool.Undef || len(theory.contains) != 0 || theory.freshID != 0 {
			t.Fatal("constant containment did not finish before symbolic registration", value, test)
		}
	}
}

func TestSMTStringSearchSetupBorrowsKernelAssertions(t *testing.T) {
	m := astTestManager(t)
	kernel := smtTestKernel(t, m)
	state := kernel.internalizer
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTStringTheory(family, nil)
	astTestMust(t, err)
	arithmeticFamily, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	arithmetic, err := NewSMTTheoryArith(arithmeticFamily, true)
	astTestMust(t, err)
	astTestMust(t, kernel.RegisterTheory(arithmetic))
	astTestMust(t, kernel.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-search-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "str-search-y"), sort)
	astTestMust(t, err)
	prefix, err := m.MkSequencePrefix(x, y)
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(prefix, ast.ASTHandle{}))
	astTestMust(t, state.Internalize(prefix, true))
	astTestMust(t, state.InitializeTheorySearch())
	if !theory.searchStarted || theory.CanPropagate() {
		t.Fatal("search initialization did not finish its source worklists")
	}
	if _, found := theory.axiomatized[prefix]; !found {
		t.Fatal("borrowed root assertion was not set up")
	}
	astTestMust(t, kernel.Push())
	contains, err := m.MkSequenceContains(y, x)
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(contains, ast.ASTHandle{}))
	astTestMust(t, state.Internalize(contains, true))
	astTestMust(t, state.InitializeTheorySearch())
	if theory.containsByPair[[2]ast.ASTHandle{y, x}] != contains {
		t.Fatal("search traversal used a stale pre-append assertion slice")
	}
	astTestMust(t, kernel.Pop(1))
	if state.assertedFormulas != &kernel.assertions || len(*state.assertedFormulas) != 1 {
		t.Fatal("borrowed formula owner did not follow kernel scope rollback")
	}
	astTestMust(t, state.InitializeTheorySearch())
	if theory.CanPropagate() {
		t.Fatal("repeated search left pending supported relation axioms")
	}
	if status, err := theory.FinalCheck(); err != nil || status != SMTFinalCheckContinue {
		t.Fatal("unassigned relation variables did not continue into tester assignment", status, err)
	}
}
