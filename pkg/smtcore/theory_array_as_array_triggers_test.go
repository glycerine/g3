package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTFullArrayAsArrayReadAxiomsUseConstructorMergeTrigger(t *testing.T) {
	for _, readBeforeMerge := range []bool{true, false} {
		t.Run(fmt.Sprint(readBeforeMerge), func(t *testing.T) {
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
			alias, err := m.MkConst(astTestSymbol(t, m, "as-array-trigger-alias"), sort)
			astTestMust(t, err)
			function, err := m.MkFuncDecl(astTestSymbol(t, m, "as-array-trigger-function"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			constructor, err := m.MkAsArray(function)
			astTestMust(t, err)
			index, err := m.MkTrue()
			astTestMust(t, err)
			read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{alias, index})
			astTestMust(t, err)
			astTestPin(t, m, read)
			astTestMust(t, state.Internalize(alias, false))
			astTestMust(t, state.Internalize(constructor, false))
			aliasVariable := state.context.enodes.get(alias).TheoryVar(family)
			constructorVariable := state.context.enodes.get(constructor).TheoryVar(family)
			if readBeforeMerge {
				astTestMust(t, state.Internalize(read, false))
				astTestMust(t, theory.addParentSelect(aliasVariable, state.context.enodes.get(read)))
			}
			astTestMust(t, theory.NewEquality(constructorVariable, aliasVariable))
			if !readBeforeMerge {
				astTestMust(t, state.Internalize(read, false))
				astTestMust(t, theory.addParentSelect(aliasVariable, state.context.enodes.get(read)))
			}
			want := uint32(0)
			if readBeforeMerge {
				want = 1
			}
			if theory.statistics.selectAsArray != want {
				t.Fatal("as-array reads did not follow native constructor-merge scheduling", readBeforeMerge, theory.statistics.selectAsArray, want)
			}
			if !theory.unsupported {
				t.Fatal("as-array lost the source incompleteness guard")
			}
		})
	}
}
