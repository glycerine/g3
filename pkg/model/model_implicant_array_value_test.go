package model_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestModelImplicantArrayValueDropsOnlyInnermostDefaultStores(t *testing.T) {
	for _, dimensions := range []int{1, 2} {
		for _, tc := range []struct {
			name     string
			values   []bool
			keepFrom int
		}{
			{"all-default", []bool{false, false, false}, 3},
			{"inner-default", []bool{false, false, true}, 2},
			{"outer-default", []bool{false, true, false}, 1},
			{"no-default", []bool{true, true, false}, 0},
		} {
			t.Run(fmt.Sprintf("%d/%s", dimensions, tc.name), func(t *testing.T) {
				m, md, implicant := modelImplicantFixture(t)
				family, err := m.RegisterArrayPlugin()
				astTestMust(t, err)
				zero, err := m.ArithmeticIntegerNumeralText("0")
				astTestMust(t, err)
				integer, err := m.GetSort(zero)
				astTestMust(t, err)
				boolean := mustBoolSort(t, m)
				domains := make([]ast.ASTHandle, dimensions)
				for i := range domains {
					domains[i] = integer
				}
				sort, err := m.MkArraySort(ast.SliceArguments(domains), boolean)
				astTestMust(t, err)
				yes, err := m.MkTrue()
				astTestMust(t, err)
				no, err := m.MkFalse()
				astTestMust(t, err)
				base, err := m.MkConstArray(sort, no)
				astTestMust(t, err)
				astTestPin(t, m, base)
				values := make([]ast.ASTHandle, len(tc.values))
				for i, yesValue := range tc.values {
					values[i] = no
					if yesValue {
						values[i] = yes
					}
				}
				indices := make([][]ast.ASTHandle, len(values))
				store := func(base ast.ASTHandle, i int) ast.ASTHandle {
					args := append([]ast.ASTHandle{base}, indices[i]...)
					args = append(args, values[i])
					h, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, args)
					astTestMust(t, err)
					astTestPin(t, m, h)
					return h
				}
				input := base
				for i := range values {
					indices[i] = make([]ast.ASTHandle, dimensions)
					for j := range indices[i] {
						indices[i][j], err = m.ArithmeticIntegerNumeralText(strconv.Itoa(i + 10*j))
						astTestMust(t, err)
					}
					input = store(input, i)
				}
				variable, err := m.MkFreshConst("implicant-array-value", sort)
				astTestMust(t, err)
				astTestPin(t, m, variable)
				decl, err := m.GetDecl(variable)
				astTestMust(t, err)
				astTestMust(t, md.Core().RegisterConstant(decl, input))
				expected := base
				for i := tc.keepFrom; i < len(values); i++ {
					expected = store(expected, i)
				}
				before := m.Stats().LiveReferences
				for _, declaration := range []bool{false, true} {
					var result ast.ASTRef
					if declaration {
						result, err = implicant.EvalDeclaration(md, decl)
					} else {
						result, err = implicant.EvalExpr(md, variable)
					}
					astTestMust(t, err)
					actual, err := m.RefGet(result)
					astTestMust(t, err)
					if actual != expected {
						t.Fatal("wrong source-normalized array value", tc.name, declaration, actual, expected)
					}
					astTestMust(t, m.DelRef(&result))
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("array evaluation leaked reference slots", got, before)
					}
				}
				original, err := md.Core().ConstantInterpretation(decl)
				astTestMust(t, err)
				if original != input {
					t.Fatal("array normalization modified the model interpretation")
				}
			})
		}
	}
}
