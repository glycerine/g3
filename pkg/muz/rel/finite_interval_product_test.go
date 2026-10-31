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

func TestFiniteIntervalProductGarbageCollectionAndReuse(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	value, err := f.ast.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	astTestPin(t, f.ast, value)
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for i := 0; i < 3; i++ {
		astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth, value}))
	}
	if empty, err := product.Empty(); err != nil || empty || len(product.available) != 2 {
		t.Fatalf("collect stale inner relations: empty=%v, free=%v, err=%v", empty, product.available, err)
	}
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.falsehood, value}))
	if len(product.intervals) != 3 || len(product.available) != 1 {
		t.Fatal("available inner index was not reused")
	}
	rows, err := product.table.Facts()
	astTestMust(t, err)
	for _, row := range rows {
		product.intervals[row[len(row)-1]].values.setEmpty()
	}
	if empty, err := product.Empty(); err != nil || !empty || len(product.intervals) != 0 {
		t.Fatalf("empty inner relations survived collection: empty=%v, err=%v", empty, err)
	}
}

func TestFiniteIntervalProductFilterEqual(t *testing.T) {
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
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, f.boolean}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{one, f.truth}, {three, f.truth}, {three, f.falsehood}} {
		astTestMust(t, product.AddFact(fact))
	}
	astTestMust(t, product.FilterEqual(0, two))
	if found, err := product.ContainsFact([]ast.ASTHandle{two, f.truth}); err != nil || !found {
		t.Fatal("interval equality", found, err)
	}
	if found, err := product.ContainsFact([]ast.ASTHandle{three, f.falsehood}); err != nil || found {
		t.Fatal("interval equality left incompatible fact", found, err)
	}
	astTestMust(t, product.FilterEqual(1, f.falsehood))
	if empty, err := product.Empty(); err != nil || !empty {
		t.Fatal("finite equality failed to remove other key", empty, err)
	}
	if err = product.FilterEqual(2, two); err == nil {
		t.Fatal("invalid column accepted")
	}
}

func TestFiniteIntervalProductUnionAndDelta(t *testing.T) {
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
	one, two, three, five, nine := num("1"), num("2"), num("3"), num("5"), num("9")
	create := func() *FiniteIntervalProduct {
		p, e := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, p.Close()) })
		return p
	}
	target, source, delta := create(), create(), create()
	astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth, one}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, three}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, five}))
	astTestMust(t, delta.AddFact([]ast.ASTHandle{f.falsehood, nine}))
	astTestMust(t, target.Union(source, delta))
	for _, check := range []struct {
		p    *FiniteIntervalProduct
		fact []ast.ASTHandle
		want bool
	}{
		{target, []ast.ASTHandle{f.truth, two}, true},
		{target, []ast.ASTHandle{f.falsehood, five}, true},
		{target, []ast.ASTHandle{f.falsehood, nine}, false},
		{delta, []ast.ASTHandle{f.truth, two}, true},
		{delta, []ast.ASTHandle{f.falsehood, five}, true},
		{delta, []ast.ASTHandle{f.falsehood, nine}, true},
	} {
		got, err := check.p.ContainsFact(check.fact)
		if err != nil || got != check.want {
			t.Fatalf("union fact %v: got %v, err %v", check.fact, got, err)
		}
	}
	if err := target.Union(source, target); err == nil {
		t.Fatal("aliased delta accepted")
	}
	astTestMust(t, target.Union(target, nil))
	if found, err := target.ContainsFact([]ast.ASTHandle{f.truth, two}); err != nil || !found {
		t.Fatal("self union changed contents", found, err)
	}
}

func TestFiniteIntervalProductProjection(t *testing.T) {
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
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer, f.boolean}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth, one, f.truth}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.falsehood, three, f.truth}))
	projected, err := product.Project([]uint32{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if found, err := projected.ContainsFact([]ast.ASTHandle{two, f.truth}); err != nil || !found {
		t.Fatal("projected keys did not merge interval hulls", found, err)
	}
	if found, err := projected.ContainsFact([]ast.ASTHandle{two, f.falsehood}); err != nil || found {
		t.Fatal("projection introduced a different finite key", found, err)
	}
	finite, err := product.Project([]uint32{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, finite.Close()) }()
	if found, err := finite.ContainsFact([]ast.ASTHandle{f.truth, f.truth}); err != nil || !found {
		t.Fatal("projected arithmetic column lost finite fact", found, err)
	}
	all, err := product.Project([]uint32{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, all.Close()) }()
	if found, err := all.ContainsFact(nil); err != nil || !found {
		t.Fatal("projection to zero columns lost fact", found, err)
	}
	if _, err = product.Project([]uint32{1, 1}); err == nil {
		t.Fatal("duplicate projected columns accepted")
	}
}

func TestFiniteIntervalProductFilterIdentical(t *testing.T) {
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
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer, f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for _, fact := range [][]ast.ASTHandle{
		{f.truth, one, f.truth, two},
		{f.truth, three, f.truth, two},
		{f.truth, two, f.falsehood, two},
	} {
		astTestMust(t, product.AddFact(fact))
	}
	astTestMust(t, product.FilterIdentical([]uint32{0, 2}))
	astTestMust(t, product.FilterIdentical([]uint32{1, 3}))
	if found, err := product.ContainsFact([]ast.ASTHandle{f.truth, two, f.truth, two}); err != nil || !found {
		t.Fatal("identical filters omitted matching tuple", found, err)
	}
	if found, err := product.ContainsFact([]ast.ASTHandle{f.truth, one, f.truth, two}); err != nil || found {
		t.Fatal("interval equality did not enforce identical values", found, err)
	}
	if found, err := product.ContainsFact([]ast.ASTHandle{f.truth, two, f.falsehood, two}); err != nil || found {
		t.Fatal("table equality did not remove mismatched key", found, err)
	}
	if err := product.FilterIdentical([]uint32{0, 1}); err == nil {
		t.Fatal("mismatched sorts accepted")
	}
}

func TestFiniteIntervalProductCloneUsesSourceVectorCopy(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	one, err := f.ast.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	astTestPin(t, f.ast, one)
	three, err := f.ast.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	astTestPin(t, f.ast, three)
	p, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	astTestMust(t, p.AddFact([]ast.ASTHandle{one, one}))
	astTestMust(t, p.AddFact([]ast.ASTHandle{three, three}))
	astTestMust(t, p.FilterIdentical([]uint32{0, 1}))
	if found, err := p.ContainsFact([]ast.ASTHandle{one, three}); err != nil || found {
		t.Fatal("source relation did not enforce equivalent columns", found, err)
	}
	clone, err := p.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	if found, err := clone.ContainsFact([]ast.ASTHandle{one, three}); err != nil || !found {
		t.Fatal("source vector copy did not split equivalence classes", found, err)
	}
}

func TestFiniteIntervalProductRename(t *testing.T) {
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
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth, one, two}))
	renamed, err := product.Rename([]uint32{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, renamed.Close()) }()
	if found, err := renamed.ContainsFact([]ast.ASTHandle{one, two, f.truth}); err != nil || !found {
		t.Fatal("rename across finite and interval columns", found, err)
	}
	intervals, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, integer, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, intervals.Close()) }()
	astTestMust(t, intervals.AddFact([]ast.ASTHandle{one, two, three}))
	rotated, err := intervals.Rename([]uint32{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, rotated.Close()) }()
	if found, err := rotated.ContainsFact([]ast.ASTHandle{three, one, two}); err != nil || !found {
		t.Fatal("source inner permutation cycle", found, err)
	}
	if _, err := product.Rename([]uint32{0, 0}); err == nil {
		t.Fatal("duplicate cycle column accepted")
	}
}

func TestFiniteIntervalProductFull(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	value, err := f.ast.ArithmeticIntegerNumeralText("-29")
	astTestMust(t, err)
	astTestPin(t, f.ast, value)
	full, err := NewFullFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, full.Close()) }()
	for _, b := range []ast.ASTHandle{f.falsehood, f.truth} {
		found, err := full.ContainsFact([]ast.ASTHandle{b, value})
		if err != nil || !found {
			t.Fatal("full product omitted fact", b, found, err)
		}
	}
	innerOnly, err := NewFullFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, innerOnly.Close()) }()
	if found, err := innerOnly.ContainsFact([]ast.ASTHandle{value}); err != nil || !found {
		t.Fatal("full product with no table columns", found, err)
	}
	if _, err = NewFullFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean}, nil); err == nil {
		t.Fatal("unsupported wide source table complement accepted")
	}
}
