package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTFullArrayCanonicalConstructorDefaultAxioms(t *testing.T) {
	for _, kind := range []string{"const", "map", "as-array", "finite-store", "large-store"} {
		t.Run(kind, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			state.theoryRewriter = smtQuickTheoryRewriter(t, m)
			theory, err := NewSMTArrayTheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			indexSort := boolean
			if kind == "as-array" || kind == "large-store" {
				indexSort, err = m.MkUninterpretedSort(astTestSymbol(t, m, "default-constructor-index"), nil)
				astTestMust(t, err)
			}
			sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolean)
			astTestMust(t, err)
			yes, err := m.MkTrue()
			astTestMust(t, err)
			no, err := m.MkFalse()
			astTestMust(t, err)
			constructor, err := m.MkConstArray(sort, yes)
			astTestMust(t, err)
			var base, function ast.ASTHandle
			switch kind {
			case "map":
				x := smtRelevancyBool(t, m, "default-map-and-left")
				y := smtRelevancyBool(t, m, "default-map-and-right")
				andXY, err := m.MkAnd(x, y)
				astTestMust(t, err)
				function, err = m.GetDecl(andXY)
				astTestMust(t, err)
				base = constructor
				constructor, err = m.MkArrayMap(function, ast.SliceArguments([]ast.ASTHandle{base, base}))
				astTestMust(t, err)
			case "as-array":
				function, err = m.MkFuncDecl(astTestSymbol(t, m, "default-array-function"), []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				constructor, err = m.MkAsArray(function)
				astTestMust(t, err)
			case "finite-store", "large-store":
				base, err = m.MkConstArray(sort, no)
				astTestMust(t, err)
				index := yes
				if kind == "large-store" {
					index, err = m.MkConst(astTestSymbol(t, m, "default-store-index"), indexSort)
					astTestMust(t, err)
				}
				constructor, err = m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, yes})
				astTestMust(t, err)
			}
			astTestMust(t, state.Internalize(constructor, false))
			defaultTerm, err := m.MkFamilyApp(family, ast.ASTArrayOpDefault, nil, []ast.ASTHandle{constructor})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(defaultTerm, false))
			value := yes
			switch kind {
			case "map":
				// Rewriting and(default(base), default(base)) removes the
				// duplicate Boolean argument; default(const) itself is not
				// a rewrite rule in the retained C++ array rewriter.
				value, err = m.MkFamilyApp(family, ast.ASTArrayOpDefault, nil, []ast.ASTHandle{base})
				astTestMust(t, err)
			case "as-array":
				epsilon := theory.epsilons[indexSort]
				if epsilon.IsNull() {
					t.Fatal("as-array default did not create a domain epsilon")
				}
				value, err = m.MkApp(function, []ast.ASTHandle{epsilon})
				astTestMust(t, err)
			case "finite-store", "large-store":
				value, err = m.MkFamilyApp(family, ast.ASTArrayOpDefault, nil, []ast.ASTHandle{base})
				astTestMust(t, err)
				if kind == "finite-store" {
					epsilon := theory.epsilons[boolean]
					if epsilon.IsNull() {
						t.Fatal("finite store default did not create an epsilon")
					}
					condition, err := m.MkEq(epsilon, yes)
					astTestMust(t, err)
					value, err = m.MkIte(condition, yes, value)
					astTestMust(t, err)
				} else if len(theory.epsilons) != 0 {
					t.Fatal("large-domain store created an unnecessary epsilon")
				}
			}
			pair := smtArrayExpressionPair{left: defaultTerm, right: value}
			if kind == "const" || kind == "as-array" {
				pair = smtArrayExpressionPair{left: value, right: defaultTerm}
			}
			if !theory.equalityAxioms[pair] {
				t.Fatal("default constructor equality has wrong operands or cache order", kind)
			}
			before := theory.statistics
			added, err := theory.instantiateConstructorDefault(constructor)
			astTestMust(t, err)
			if added || before != theory.statistics {
				t.Fatal("duplicate default constructor fingerprint emitted another axiom")
			}
		})
	}
}

func TestSMTFullArrayDefaultFingerprintsAndEpsilonsAreScoped(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	base, err := m.MkConstArray(sort, no)
	astTestMust(t, err)
	store, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, yes, yes})
	astTestMust(t, err)
	astTestPin(t, m, store)
	astTestMust(t, state.Internalize(base, false))
	baseNode := state.context.enodes.get(base)
	var previousEpsilon ast.ASTHandle
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, state.PushScope())
		astTestMust(t, state.Internalize(store, false))
		added, err := theory.instantiateConstructorDefault(store)
		astTestMust(t, err)
		if !added || len(theory.defaultFingerprintTrail) != 2 {
			t.Fatal("child store default was not instantiated")
		}
		epsilon := theory.epsilons[boolean]
		astTestPin(t, m, epsilon)
		if epsilon.IsNull() || epsilon == previousEpsilon {
			t.Fatal("popped epsilon was reused in the new scope")
		}
		previousEpsilon = epsilon
		astTestMust(t, state.PopScopes(1))
		if len(theory.defaultFingerprintTrail) != 1 || !theory.defaultFingerprints[baseNode] || len(theory.epsilons) != 0 || len(theory.equalityAxioms) != 0 {
			t.Fatal("pop did not preserve base fingerprints while clearing child state and all equality pairs")
		}
		added, err = theory.instantiateConstructorDefault(base)
		astTestMust(t, err)
		if added {
			t.Fatal("base constructor fingerprint was lost on pop")
		}
	}
}
