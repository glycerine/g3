package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestEmptyRelationsExportFalse(t *testing.T) {
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
	for name, makeFormula := range map[string]func() (ast.ASTRef, error){
		"bound":    func() (ast.ASTRef, error) { return bound.ToFormula(m, sorts) },
		"interval": func() (ast.ASTRef, error) { return interval.ToFormula(m, sorts) },
		"product":  func() (ast.ASTRef, error) { return product.ToFormula(m, sorts) },
		"empty-product": func() (ast.ASTRef, error) {
			return NewIntervalBoundProduct(2, true).ToFormula(m, sorts)
		},
	} {
		ref, err := makeFormula()
		astTestMust(t, err)
		h, err := m.RefGet(ref)
		astTestMust(t, err)
		if !m.IsFalse(h) {
			t.Errorf("%s empty relation exported a non-false formula", name)
		}
		astTestMust(t, m.DelRef(&ref))
	}
}
