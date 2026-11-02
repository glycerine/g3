package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalSieveProductFormulaUsesExactOuterColumnIndices(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	product, err := NewSparseIntervalSieveProduct(relations,
		[]ast.ASTHandle{f.boolean, integer, f.boolean, integer},
		[]string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	two, four, six, eight, one := numeral("2"), numeral("4"), numeral("6"), numeral("8"), numeral("1")
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.falsehood, two, f.truth, four}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth, six, f.falsehood, eight}))

	formula, err := product.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	handle, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	theory := tableRelationTheoryRewriter(t, f.ast)
	evaluate := func(column0, column1, column2, column3 ast.ASTHandle) bool {
		out, cause := f.ast.NewRef(ast.ASTHandle{})
		astTestMust(t, cause)
		defer func() { astTestMust(t, f.ast.DelRef(&out)) }()
		// RewriteWithBindings uses standard de Bruijn order: variable zero is last.
		astTestMust(t, theory.RewriteWithBindings(handle, []ast.ASTHandle{column3, column2, column1, column0}, out))
		answer, cause := f.ast.RefGet(out)
		astTestMust(t, cause)
		return f.ast.IsTrue(answer)
	}
	if !evaluate(f.falsehood, six, f.truth, eight) {
		t.Fatal("formula lost the independent product cross-combination")
	}
	if evaluate(f.falsehood, six, f.falsehood, eight) {
		t.Fatal("formula mapped a finite component to the wrong outer columns")
	}
	if evaluate(f.falsehood, one, f.truth, eight) {
		t.Fatal("formula mapped an interval component to the wrong outer columns")
	}
}
