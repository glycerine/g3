package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArrayEqualityDispatchInTheoryAndSimplifiedApp(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			m, context, globals, local := boolRewriterConfigFixture(t)
			astTestMust(t, m.RegisterDeclarationPlugins())
			key := "expand_store_eq"
			astTestMust(t, context.SetBoolName(local, &key, enabled))
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			boolSort, err := m.MkBoolSort()
			astTestMust(t, err)
			sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolSort}), boolSort)
			astTestMust(t, err)
			base, err := m.MkConst(astTestSymbol(t, m, "dispatch-array"), sort)
			astTestMust(t, err)
			index := goalTestAtom(t, m, "dispatch-index")
			value := goalTestAtom(t, m, "dispatch-value")
			stored, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, value})
			astTestMust(t, err)
			equality, err := m.MkEq(stored, base)
			astTestMust(t, err)
			astTestPin(t, m, equality)
			selected, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{base, index})
			astTestMust(t, err)
			expected, err := m.MkEq(value, selected)
			astTestMust(t, err)
			astTestPin(t, m, expected)
			theory, err := NewTheoryRewriter(m, context, globals, local)
			astTestMust(t, err)
			defer func() { astTestMust(t, theory.Close()) }()
			out := astTestPin(t, m, ast.ASTHandle{})
			astTestMust(t, theory.Rewrite(equality, out))
			actual, err := m.RefGet(out)
			astTestMust(t, err)
			if enabled {
				want := astTestPin(t, m, ast.ASTHandle{})
				astTestMust(t, theory.Rewrite(expected, want))
				h, e := m.RefGet(want)
				astTestMust(t, e)
				if actual != h {
					t.Fatal("theory rewriter did not dispatch array equality")
				}
			} else if actual != equality {
				t.Fatal("disabled array equality expansion changed the expression")
			}
			simple, err := NewSimplifiedApp(m, context, globals, local)
			astTestMust(t, err)
			defer func() { astTestMust(t, simple.Close()) }()
			decl, err := m.GetDecl(equality)
			astTestMust(t, err)
			status, err := simple.MkCore(decl, ast.SliceArguments([]ast.ASTHandle{stored, base}), out)
			astTestMust(t, err)
			if enabled && status != RewriteFull {
				t.Fatal("simplified app did not dispatch array equality", status)
			}
			if !enabled && status != RewriteFailed {
				t.Fatal("disabled simplified app expanded array equality", status)
			}
		})
	}
}
