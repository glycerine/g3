package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteIntervalProductJoin(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	num := func(text string) ast.ASTHandle {
		value, e := f.ast.ArithmeticIntegerNumeralText(text)
		astTestMust(t, e)
		astTestPin(t, f.ast, value)
		return value
	}
	one, two, three := num("1"), num("2"), num("3")
	left, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, left.Close()) }()
	right, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, f.boolean}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, right.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{f.truth, one}, {f.truth, three}, {f.falsehood, three}} {
		astTestMust(t, left.AddFact(fact))
	}
	for _, fact := range [][]ast.ASTHandle{{two, f.truth}, {three, f.falsehood}} {
		astTestMust(t, right.AddFact(fact))
	}
	joined, err := JoinFiniteIntervalProducts(left, right, []uint32{0, 1}, []uint32{1, 0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{
		{[]ast.ASTHandle{f.truth, two, two, f.truth}, true},
		{[]ast.ASTHandle{f.falsehood, three, three, f.falsehood}, true},
		{[]ast.ASTHandle{f.truth, one, two, f.truth}, false},
		{[]ast.ASTHandle{f.truth, two, two, f.falsehood}, false},
	} {
		found, e := joined.ContainsFact(tc.fact)
		if e != nil || found != tc.want {
			t.Fatalf("joined fact %v: %v, %v", tc.fact, found, e)
		}
	}
	projected, err := joined.Project([]uint32{2, 3})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if found, err := projected.ContainsFact([]ast.ASTHandle{f.truth, two}); err != nil || !found {
		t.Fatal("joined projection lost matching tuple", found, err)
	}
	empty, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, f.boolean}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	emptyJoin, err := JoinFiniteIntervalProducts(left, empty, nil, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, emptyJoin.Close()) }()
	if isEmpty, err := emptyJoin.Empty(); err != nil || !isEmpty {
		t.Fatal("join with empty product", isEmpty, err)
	}
	if _, err = JoinFiniteIntervalProducts(left, right, []uint32{0}, []uint32{0}); err == nil {
		t.Fatal("mismatched join sorts accepted")
	}
	if _, err = JoinFiniteIntervalProducts(left, right, []uint32{0}, nil); err == nil {
		t.Fatal("mismatched column lists accepted")
	}
}
