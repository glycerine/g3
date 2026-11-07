package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTFullArrayEqualityCacheOrderedPairsAndScopeReset(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	_, values := smtArrayQueueIndices(t, m, "array-equality-cache-", 2)
	for _, pair := range [][2]ast.ASTHandle{{values[0], values[1]}, {values[1], values[0]}} {
		added, err := theory.tryAssignArrayEquality(pair[0], pair[1])
		astTestMust(t, err)
		if !added {
			t.Fatal("ordered pair was treated as a symmetric cache entry")
		}
		added, err = theory.tryAssignArrayEquality(pair[0], pair[1])
		astTestMust(t, err)
		if added {
			t.Fatal("duplicate ordered pair emitted another unit")
		}
	}
	if len(theory.equalityAxioms) != 2 {
		t.Fatal("ordered cache size", len(theory.equalityAxioms))
	}
	atom, err := state.mkEqualityAtom(values[0], values[1])
	astTestMust(t, err)
	variable, found := state.BoolVar(atom)
	if !found || !state.relevancy.IsRelevant(atom) || state.boolExpr[variable] != atom {
		t.Fatal("cached equality atom was not internalized and marked relevant")
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.PushScope())
	astTestMust(t, state.PopScopes(2))
	if len(theory.equalityAxioms) != 0 {
		t.Fatal("scope pop retained pre-scope equality cache entries")
	}
	added, err := theory.tryAssignArrayEquality(values[0], values[1])
	astTestMust(t, err)
	if !added {
		t.Fatal("scope pop did not permit reasserting the base pair")
	}
	reused, found := state.BoolVar(atom)
	if !found || reused != variable {
		t.Fatal("clearing the pair cache discarded the base Boolean atom")
	}
}

func TestSMTFullArrayCanonicalConstructorReadAxioms(t *testing.T) {
	for _, kind := range []string{"const", "map", "as-array"} {
		t.Run(kind, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			state.theoryRewriter = smtQuickTheoryRewriter(t, m)
			theory, err := NewSMTArrayTheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))
			indexSort, indices := smtArrayQueueIndices(t, m, "canonical-constructor-", 1)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolean)
			astTestMust(t, err)
			alias, err := m.MkConst(astTestSymbol(t, m, "canonical-alias"), sort)
			astTestMust(t, err)
			yes, err := m.MkTrue()
			astTestMust(t, err)
			constructor, err := m.MkConstArray(sort, yes)
			astTestMust(t, err)
			value := yes
			switch kind {
			case "map":
				x := smtRelevancyBool(t, m, "canonical-map-not-argument")
				notX, err := m.MkNot(x)
				astTestMust(t, err)
				function, err := m.GetDecl(notX)
				astTestMust(t, err)
				constructor, err = m.MkArrayMap(function, ast.SliceArguments([]ast.ASTHandle{constructor}))
				astTestMust(t, err)
				value, err = m.MkFalse()
				astTestMust(t, err)
			case "as-array":
				function, err := m.MkFuncDecl(astTestSymbol(t, m, "canonical-array-function"), []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				constructor, err = m.MkAsArray(function)
				astTestMust(t, err)
				value, err = m.MkApp(function, indices)
				astTestMust(t, err)
			}
			astTestMust(t, state.Internalize(constructor, false))
			astTestMust(t, state.Internalize(alias, false))
			astTestMust(t, state.addEquality(state.context.enodes.get(alias), state.context.enodes.get(constructor), NewSMTEqualityAxiom()))
			observer, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{alias, indices[0]})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(observer, false))
			added, err := theory.instantiateConstructorRead(observer, constructor)
			astTestMust(t, err)
			selected, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{constructor, indices[0]})
			astTestMust(t, err)
			if !added || !theory.equalityAxioms[smtArrayExpressionPair{left: selected, right: value}] {
				t.Fatal("constructor read did not assert its canonical equality", kind, added)
			}
			if theory.equalityAxioms[smtArrayExpressionPair{left: observer, right: value}] {
				t.Fatal("alias observer was used in place of the constructor read")
			}
			atom, err := state.mkEqualityAtom(selected, value)
			astTestMust(t, err)
			if !state.relevancy.IsRelevant(atom) || state.context.enodes.get(selected) == nil {
				t.Fatal("canonical constructor read was not internalized or relevant")
			}
			before := theory.statistics
			added, err = theory.instantiateConstructorRead(selected, constructor)
			astTestMust(t, err)
			if added || before != theory.statistics {
				t.Fatal("alias and direct read did not share their constructor/index fingerprint")
			}
		})
	}
}
