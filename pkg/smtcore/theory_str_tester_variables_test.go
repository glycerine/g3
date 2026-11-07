package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringTesterNamesRolesAndOwnership(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "word with space")
	astTestMust(t, state.PushScope())
	before := len(theory.basicTodo)
	length, err := theory.freshStringLengthTester(word, 3)
	astTestMust(t, err)
	value, err := theory.freshStringValueTester(word, 7, 2)
	astTestMust(t, err)
	second, err := theory.freshStringLengthTester(word, 4)
	astTestMust(t, err)
	for _, test := range []struct {
		variable ast.ASTHandle
		name     string
		length   bool
	}{
		{length, "$$_len_|word with space|_3_0!tmp0", true},
		{value, "$$_val_|word with space|_7_2_0!tmp1", false},
		{second, "$$_len_|word with space|_4_1!tmp2", true},
	} {
		decl, err := m.GetDecl(test.variable)
		astTestMust(t, err)
		symbol, err := m.GetASTName(decl)
		astTestMust(t, err)
		name, _, err := m.Symbols.BareString(symbol)
		astTestMust(t, err)
		if name != test.name {
			t.Fatalf("tester name %q, want %q", name, test.name)
		}
		_, isLength := theory.lengthTestVariables[test.variable]
		_, isValue := theory.valueTestVariables[test.variable]
		_, internal := theory.internalVariables[test.variable]
		_, variable := theory.variableSet[test.variable]
		_, scoped := theory.variableScopes[theory.level][test.variable]
		if isLength != test.length || isValue == test.length || !internal || !variable || !scoped || state.context.enodes.get(test.variable) == nil {
			t.Fatal("tester lost native role/internalization/scope membership")
		}
		roots := 0
		for _, ref := range theory.refs {
			term, err := m.RefGet(ref)
			astTestMust(t, err)
			if term == test.variable {
				roots++
			}
		}
		if roots != 2 {
			t.Fatal("tester must keep both direct lifetime roots", roots)
		}
	}
	if len(theory.basicTodo) != before+3 || theory.testers.lengthID != 2 || theory.testers.valueID != 1 || theory.freshID != 3 {
		t.Fatal("tester queues or independent counters changed")
	}
	astTestMust(t, state.PopScopes(1))
	if _, ok := theory.internalVariables[length]; !ok {
		t.Fatal("scope pop erased lifetime tester membership")
	}
	if _, err := m.GetKind(value); err != nil {
		t.Fatal("scope pop released tester", err)
	}
	fresh, err := theory.Fresh()
	astTestMust(t, err)
	defer func() { astTestMust(t, fresh.Close()) }()
	if fresh.testers.lengthID != 0 || fresh.testers.valueID != 0 || fresh.freshID != 0 || len(fresh.lengthTestVariables) != 0 || len(fresh.valueTestVariables) != 0 {
		t.Fatal("fresh theory copied tester state")
	}
}

func TestSMTStringTesterFormattingFailurePreservesCounters(t *testing.T) {
	_, theory, _, _ := stringLibraryFixture(t)
	if _, err := theory.freshStringLengthTester(ast.ASTHandle{}, 1); err == nil {
		t.Fatal("invalid subject accepted")
	}
	if _, err := theory.freshStringValueTester(ast.ASTHandle{}, 1, 1); err == nil {
		t.Fatal("invalid subject accepted")
	}
	if theory.testers.lengthID != 0 || theory.testers.valueID != 0 || theory.freshID != 0 {
		t.Fatal("formatting failure consumed tester IDs")
	}
}
