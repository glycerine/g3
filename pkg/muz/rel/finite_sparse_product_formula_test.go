package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductToFormulaMatchesRows(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-formula-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{two, zero}))
	formula, err := product.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	formulaHandle, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	theory := tableRelationTheoryRewriter(t, f.ast)
	evaluate := func(first, second ast.ASTHandle) bool {
		out, e := f.ast.NewRef(ast.ASTHandle{})
		astTestMust(t, e)
		defer func() { astTestMust(t, f.ast.DelRef(&out)) }()
		astTestMust(t, theory.RewriteWithBindings(formulaHandle, []ast.ASTHandle{second, first}, out))
		result, e := f.ast.RefGet(out)
		astTestMust(t, e)
		return f.ast.IsTrue(result)
	}
	for _, tc := range []struct {
		first, second ast.ASTHandle
		want          bool
	}{{zero, one, true}, {two, zero, true}, {one, zero, false}} {
		if got := evaluate(tc.first, tc.second); got != tc.want {
			t.Fatalf("formula(%v,%v) = %v, want %v", tc.first, tc.second, got, tc.want)
		}
	}
	empty, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	emptyFormula, err := empty.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&emptyFormula)) }()
	emptyHandle, err := f.ast.RefGet(emptyFormula)
	astTestMust(t, err)
	if !f.ast.IsFalse(emptyHandle) {
		t.Fatal("empty product formula is not false")
	}
}
