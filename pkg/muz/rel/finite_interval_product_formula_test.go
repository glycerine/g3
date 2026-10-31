package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteIntervalProductToFormulaSourceOrder(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	one := own(f.ast.ArithmeticIntegerNumeralText("1"))
	for _, tc := range []struct {
		name       string
		sorts      []ast.ASTHandle
		fact       []ast.ASTHandle
		firstSort  ast.ASTHandle
		firstValue ast.ASTHandle
	}{
		{"finite first", []ast.ASTHandle{f.boolean, integer}, []ast.ASTHandle{f.truth, one}, f.boolean, f.truth},
		{"interleaved source positions", []ast.ASTHandle{integer, f.boolean}, []ast.ASTHandle{one, f.truth}, integer, one},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewFiniteIntervalProduct(f.relations, tc.sorts, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, p.Close()) }()
			empty, err := p.ToFormula()
			astTestMust(t, err)
			emptyValue, err := f.ast.RefGet(empty)
			astTestMust(t, err)
			if !f.ast.IsFalse(emptyValue) {
				t.Fatal("empty product formula")
			}
			astTestMust(t, f.ast.DelRef(&empty))
			astTestMust(t, p.AddFact(tc.fact))
			formula, err := p.ToFormula()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
			actual, err := f.ast.RefGet(formula)
			astTestMust(t, err)
			keyVar := own(f.ast.MkVar(0, tc.firstSort))
			keyEq := own(f.ast.MkEq(keyVar, tc.firstValue))
			innerVar := own(f.ast.MkVar(1, integer))
			lo := own(f.ast.MkArithmeticLE(one, innerVar))
			hi := own(f.ast.MkArithmeticLE(innerVar, one))
			bounds := own(f.ast.MkAndArgs([]ast.ASTHandle{lo, hi}))
			want := own(f.ast.MkAndArgs([]ast.ASTHandle{keyEq, bounds}))
			if actual != want {
				t.Fatal("formula did not preserve source variable positions and conjunction order")
			}
		})
	}
}
