package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTArrayConstructorMergesMarkClassUpwardAndRollback(t *testing.T) {
	for _, kind := range []string{"simple-store", "full-store", "map", "const", "as-array"} {
		for _, weak := range []bool{false, true} {
			t.Run(fmt.Sprint(kind, "/", weak), func(t *testing.T) {
				m := astTestManager(t)
				family, err := m.RegisterArrayPlugin()
				astTestMust(t, err)
				_, _, state := smtInternalizerFixture(t, m, nil)
				var plugin SMTTheoryInternalizer
				if kind == "simple-store" {
					plugin, err = NewSMTSimpleArrayTheory(family)
				} else {
					plugin, err = NewSMTArrayTheory(family)
				}
				astTestMust(t, err)
				astTestMust(t, state.RegisterTheory(plugin))
				theory := smtArrayCore(plugin)
				theory.arrayConfig.Weak = weak
				boolean, err := m.MkBoolSort()
				astTestMust(t, err)
				sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
				astTestMust(t, err)
				alias, err := m.MkConst(astTestSymbol(t, m, "upward-merge-alias"), sort)
				astTestMust(t, err)
				input, err := m.MkConst(astTestSymbol(t, m, "upward-merge-input"), sort)
				astTestMust(t, err)
				yes, err := m.MkTrue()
				astTestMust(t, err)
				function, err := m.MkFuncDecl(astTestSymbol(t, m, "upward-merge-function"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				var constructor ast.ASTHandle
				switch kind {
				case "simple-store", "full-store":
					constructor, err = m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{input, yes, yes})
				case "map":
					constructor, err = m.MkArrayMap(function, ast.SliceArguments([]ast.ASTHandle{input}))
				case "const":
					constructor, err = m.MkConstArray(sort, yes)
				case "as-array":
					constructor, err = m.MkAsArray(function)
				}
				astTestMust(t, err)
				astTestMust(t, state.Internalize(alias, false))
				astTestMust(t, state.Internalize(constructor, false))
				aliasVariable := state.context.enodes.get(alias).TheoryVar(family)
				constructorVariable := state.context.enodes.get(constructor).TheoryVar(family)
				no, err := m.MkFalse()
				astTestMust(t, err)
				parentStore, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{alias, no, yes})
				astTestMust(t, err)
				astTestMust(t, state.Internalize(parentStore, false))
				read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{alias, yes})
				astTestMust(t, err)
				astTestMust(t, state.Internalize(read, false))
				astTestMust(t, theory.addParentStore(aliasVariable, state.context.enodes.get(parentStore)))
				astTestMust(t, theory.addParentSelect(aliasVariable, state.context.enodes.get(read)))
				data := theory.arrayData[aliasVariable]
				if data.propUpward {
					t.Fatal("alias was marked upward before acquiring a constructor")
				}
				astTestMust(t, state.PushScope())
				astTestMust(t, theory.NewEquality(constructorVariable, aliasVariable))
				if theory.arrayUnion.Find(uint32(aliasVariable)) != uint32(aliasVariable) {
					t.Fatal("fixture did not retain the alias as the new private root")
				}
				if data.propUpward != !weak {
					t.Fatal("constructor merge did not apply the source upward class flag", kind, weak, data.propUpward)
				}
				if kind == "simple-store" || kind == "full-store" || kind == "map" {
					inputVariable := state.context.enodes.get(input).TheoryVar(family)
					if theory.arrayData[theory.arrayUnion.Find(uint32(inputVariable))].propUpward != !weak {
						t.Fatal("new constructor inputs did not receive upward propagation")
					}
				}
				// The class flag must enable its delayed read-over-write
				// consequence for an existing non-overwriting parent store.
				before := theory.statistics.expandedAxiom2
				added, err := theory.assertDelayedStoreAxioms()
				astTestMust(t, err)
				wantAxioms := uint32(1)
				if weak {
					wantAxioms = 0
				}
				if added != !weak || theory.statistics.expandedAxiom2-before != wantAxioms {
					t.Fatal("constructor merge did not enable its delayed parent-store read axiom", weak, added, theory.statistics.expandedAxiom2-before)
				}
				astTestMust(t, state.PopScopes(1))
				if data.propUpward || len(data.stores)+len(data.maps)+len(data.consts)+len(data.asArrays) != 0 || theory.arrayUnion.Find(uint32(constructorVariable)) != uint32(constructorVariable) {
					t.Fatal("constructor merge class metadata survived scope rollback")
				}
			})
		}
	}
}
