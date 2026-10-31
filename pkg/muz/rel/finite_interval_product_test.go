package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteIntervalProductFactsAndClone(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	numeral := func(text string) ast.ASTHandle {
		h, e := f.ast.ArithmeticIntegerNumeralText(text)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	one, two, three, five := numeral("1"), numeral("2"), numeral("3"), numeral("5")
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, f.boolean}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if empty, e := product.Empty(); e != nil || !empty {
		t.Fatal("new product", empty, e)
	}
	astTestMust(t, product.AddFact([]ast.ASTHandle{one, f.truth}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{three, f.truth}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{five, f.falsehood}))
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{
		{[]ast.ASTHandle{two, f.truth}, true},
		{[]ast.ASTHandle{two, f.falsehood}, false},
		{[]ast.ASTHandle{five, f.falsehood}, true},
		{[]ast.ASTHandle{five, f.truth}, false},
	} {
		got, e := product.ContainsFact(tc.fact)
		if e != nil || got != tc.want {
			t.Fatalf("contains %v: %v, %v", tc.fact, got, e)
		}
	}
	clone, err := product.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	astTestMust(t, clone.AddFact([]ast.ASTHandle{five, f.truth}))
	original, err := product.ContainsFact([]ast.ASTHandle{five, f.truth})
	astTestMust(t, err)
	copied, err := clone.ContainsFact([]ast.ASTHandle{five, f.truth})
	astTestMust(t, err)
	if original || !copied {
		t.Fatal("clone modified original")
	}
	if err = product.AddFact([]ast.ASTHandle{f.truth, f.truth}); err == nil {
		t.Fatal("wrong sort accepted")
	}
}

func TestFiniteIntervalProductColumnSelection(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	if _, err = NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer}, []bool{true}); err == nil {
		t.Fatal("unbounded integer accepted as table column")
	}
	if _, err = NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean}, []bool{false}); err == nil {
		t.Fatal("Boolean accepted as interval column")
	}
	if _, err = NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer}, []bool{}); err == nil {
		t.Fatal("invalid mask accepted")
	}
	product, err := NewFiniteIntervalProduct(f.relations, nil, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact(nil))
	found, err := product.ContainsFact(nil)
	astTestMust(t, err)
	if !found {
		t.Fatal("zero-column fact missing")
	}
}
