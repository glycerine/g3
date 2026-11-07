package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringFreeVariableOrderPrivateRepresentativesAndRoles(t *testing.T) {
	m, theory, state, _ := stringBackpropMixedFixture(t)
	first := stringGroundingWord(t, theory, "tester-order-first")
	alias := stringGroundingWord(t, theory, "tester-order-alias")
	other := stringGroundingWord(t, theory, "tester-order-other")
	regex := stringGroundingWord(t, theory, "tester-order-regex")
	internal := stringGroundingWord(t, theory, "tester-order-internal")
	theory.regexVariables[regex] = struct{}{}
	theory.internalVariables[internal] = struct{}{}
	constant := astTestString(t, m, "constant")
	astTestMust(t, state.Internalize(constant, false))
	stringDependencyPrivateMerge(t, theory, first, alias)
	stringDependencyPrivateMerge(t, theory, first, constant)
	variables := theory.stringClassVariables(alias)
	if len(variables) != 2 || variables[0] != first || variables[1] != alias {
		t.Fatal("registered private-class variables changed", variables)
	}
	ordered, err := theory.stringFreeVariableOrder(map[ast.ASTHandle]int{first: 99, alias: 1, other: 2, regex: 1, internal: 1})
	astTestMust(t, err)
	// process_free_var excludes regex variables, but does not exclude an
	// internal candidate supplied by its input map. Counts do not prioritize.
	if len(ordered) != 3 || ordered[0] != first || ordered[1] != other || ordered[2] != internal {
		t.Fatal("native representative/role selection changed", ordered)
	}
}

func TestSMTStringFreeVariableOrderLeafBeforeUnknownStandalone(t *testing.T) {
	m, theory, state, _ := stringBackpropMixedFixture(t)
	alone := stringGroundingWord(t, theory, "tester-order-alone")
	lengthOnly := stringGroundingWord(t, theory, "tester-order-length-use")
	known := stringGroundingWord(t, theory, "tester-order-known")
	leaf := stringGroundingWord(t, theory, "tester-order-concat-leaf")
	length, err := m.MkSequenceLength(lengthOnly)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(length, false))
	astTestMust(t, theory.setUpAxioms(length))
	_ = stringDependencyConcat(t, theory, leaf, astTestString(t, m, "x"))
	stringSetLength(t, m, state, known, "3")
	ordered, err := theory.stringFreeVariableOrder(map[ast.ASTHandle]int{alone: 1, lengthOnly: 1, known: 1, leaf: 1})
	astTestMust(t, err)
	if len(ordered) != 4 || ordered[0] != known || ordered[1] != leaf || ordered[2] != alone || ordered[3] != lengthOnly {
		t.Fatal("leaf/known-length ordering or unreachable input-length cache behavior changed", ordered)
	}
}

func TestSMTStringFreeVariableOrderUsesRepresentativeDirectParents(t *testing.T) {
	m, theory, _, _ := stringBackpropMixedFixture(t)
	first := stringGroundingWord(t, theory, "tester-order-parent-first")
	alias := stringGroundingWord(t, theory, "tester-order-parent-alias")
	leaf := stringGroundingWord(t, theory, "tester-order-parent-leaf")
	stringDependencyPrivateMerge(t, theory, first, alias)
	constant := astTestString(t, m, "x")
	_ = stringDependencyConcat(t, theory, alias, constant)
	_ = stringDependencyConcat(t, theory, leaf, constant)
	ordered, err := theory.stringFreeVariableOrder(map[ast.ASTHandle]int{first: 1, alias: 1, leaf: 1})
	astTestMust(t, err)
	if len(ordered) != 2 || ordered[0] != leaf || ordered[1] != first {
		t.Fatal("selection generalized direct parents to all private-class aliases", ordered)
	}
}

func TestSMTStringFreeVariableOrderInternalizesMissingCandidate(t *testing.T) {
	m, theory, state, _ := stringBackpropMixedFixture(t)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	word, err := m.MkConst(astTestSymbol(t, m, "tester-order-missing"), sort)
	astTestMust(t, err)
	astTestPin(t, m, word)
	if state.context.enodes.get(word) != nil {
		t.Fatal("fixture candidate was already internalized")
	}
	ordered, err := theory.stringFreeVariableOrder(map[ast.ASTHandle]int{word: 1})
	astTestMust(t, err)
	if len(ordered) != 1 || ordered[0] != word || state.context.enodes.get(word) == nil {
		t.Fatal("standalone selection omitted native internalization")
	}
	if _, registered := theory.variableSet[word]; registered {
		t.Fatal("selection invented a set_up_axioms registration")
	}
}
