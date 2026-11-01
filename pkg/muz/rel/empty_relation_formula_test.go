package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestEmptyRelationFormulaSourceSemantics(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	sorts := []ast.ASTHandle{sort, sort}
	bound := NewBoundRelation(2, true)
	interval := NewIntervalEmpty(2)
	product := NewIntervalBoundProduct(2, false)
	astTestMust(t, product.bound.AddLess(0, 1))
	astTestMust(t, product.bound.AddLessEqual(1, 0))
	if !product.Empty() {
		t.Fatal("strict cycle must make product empty")
	}
	for name, test := range map[string]struct {
		makeFormula func() (ast.ASTRef, error)
		wantTrue    bool
	}{
		"bound":         {func() (ast.ASTRef, error) { return bound.ToFormula(m, sorts) }, true},
		"product-cycle": {func() (ast.ASTRef, error) { return product.ToFormula(m, sorts) }, false},
		"empty-product": {func() (ast.ASTRef, error) { return NewIntervalBoundProduct(2, true).ToFormula(m, sorts) }, true},
	} {
		ref, err := test.makeFormula()
		astTestMust(t, err)
		h, err := m.RefGet(ref)
		astTestMust(t, err)
		if m.IsTrue(h) != test.wantTrue {
			t.Errorf("%s formula is %v, want true=%v", name, h, test.wantTrue)
		}
		astTestMust(t, m.DelRef(&ref))
	}
	intervalRef, err := interval.ToFormula(m, sorts)
	astTestMust(t, err)
	intervalFormula, err := m.RefGet(intervalRef)
	astTestMust(t, err)
	if !m.IsTrue(intervalFormula) {
		t.Fatal("empty interval relation formula", intervalFormula, "want source empty-conjunction true")
	}
	astTestMust(t, m.DelRef(&intervalRef))
}
