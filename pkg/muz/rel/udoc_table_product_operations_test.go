package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func udocTableProductOperationShape(t *testing.T, f *tableRelationFixture, doc *TypedUDocRelation) *UDocTableProduct {
	t.Helper()
	finite := udocTableProductFinite(t, []uint64{2, 4})
	raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean, f.bits}, finite, nil, nil)
	return ownUDocTableProduct(t, raw, err)
}

func TestUDocTableProductAlignedUnionAccumulatesDeltaAndAllowsSourceAlias(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	target := udocTableProductOperationShape(t, f, doc)
	old := []ast.ASTHandle{values[0], f.truth, values[0]}
	added := []ast.ASTHandle{values[1], f.falsehood, values[2]}
	previous := []ast.ASTHandle{values[3], f.truth, values[3]}
	astTestMust(t, target.AddFact(old))
	raw, err := target.EmptyLike()
	source := ownUDocTableProduct(t, raw, err)
	raw, err = target.EmptyLike()
	delta := ownUDocTableProduct(t, raw, err)
	if empty, err := source.Doc().Empty(); err != nil || !empty {
		t.Fatal("EmptyLike copied nonempty DOC component", empty, err)
	}
	if count, err := source.Table().Len(); err != nil || count != 0 {
		t.Fatal("EmptyLike copied finite rows", count, err)
	}
	assertUDocSieveMask(t, source.Doc(), true, false, true)
	astTestMust(t, source.AddFact(old))
	astTestMust(t, source.AddFact(added))
	astTestMust(t, delta.AddFact(previous))
	for run := 0; run < 2; run++ {
		astTestMust(t, target.UnionFrom(source, delta))
		assertUDocTableProductFormula(t, f.ast, target, old, added)
		assertUDocTableProductFormula(t, f.ast, source, old, added)
		assertUDocTableProductFormula(t, f.ast, delta, added, previous)
		assertUDocTableProductContains(t, delta, false, old...)
	}
	astTestMust(t, target.UnionFrom(target, delta))
	assertUDocTableProductFormula(t, f.ast, target, old, added)
	assertUDocTableProductFormula(t, f.ast, delta, added, previous)
	astTestMust(t, target.UnionFrom(source, nil))
	assertUDocTableProductFormula(t, f.ast, source, old, added)
	for _, invalidDelta := range []*UDocTableProduct{target, source} {
		if err := target.UnionFrom(source, invalidDelta); err == nil {
			t.Fatal("compiler delta aliased a source or target owner")
		}
		assertUDocTableProductFormula(t, f.ast, target, old, added)
		assertUDocTableProductFormula(t, f.ast, source, old, added)
	}
}

func TestUDocTableProductDeltaCanBeEmptyWhenOnlyFiniteComponentGrows(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	target := udocTableProductOperationShape(t, f, doc)
	raw, err := target.EmptyLike()
	source := ownUDocTableProduct(t, raw, err)
	raw, err = target.EmptyLike()
	delta := ownUDocTableProduct(t, raw, err)
	old := []ast.ASTHandle{values[1], f.truth, values[2]}
	added := []ast.ASTHandle{values[1], f.falsehood, values[2]}
	astTestMust(t, target.AddFact(old))
	astTestMust(t, source.AddFact(added))
	astTestMust(t, target.UnionFrom(source, delta))
	assertUDocTableProductFormula(t, f.ast, target, old, added)
	if empty, err := delta.Empty(); err != nil || !empty {
		t.Fatal("component delta was replaced by an exact whole-product difference", empty, err)
	}
	if empty, err := delta.Doc().Empty(); err != nil || !empty {
		t.Fatal("unchanged DOC component produced a delta", empty, err)
	}
	if found, err := delta.Table().Contains(Fact{1, 0, 2}); err != nil || !found {
		t.Fatal("finite delta disappeared because DOC delta was empty", found, err)
	}
	assertUDocTableProductFormula(t, f.ast, delta)
	assertUDocTableProductFormula(t, f.ast, source, added)
}

func TestUDocTableProductUnionRejectsDifferentMasksAndLayoutsBeforeMutation(t *testing.T) {
	f, factory, doc, values := udocTableProductFixture(t)
	target := udocTableProductOperationShape(t, f, doc)
	old := []ast.ASTHandle{values[0], f.truth, values[0]}
	added := []ast.ASTHandle{values[1], f.falsehood, values[2]}
	astTestMust(t, target.AddFact(old))
	raw, err := target.EmptyLike()
	source := ownUDocTableProduct(t, raw, err)
	astTestMust(t, source.AddFact(added))
	inner, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
	astTestMust(t, err)
	// Source product alignment compares concrete sieve masks, not just the
	// common outer signature. This explicit mask ignores a supported BV sort.
	sieve, err := AdoptUDocSieve(target.Sorts(), []bool{true, false, false}, inner)
	astTestMust(t, err)
	table, err := source.Table().CloneFinite()
	astTestMust(t, err)
	wrongMask := ownUDocTableProduct(t, &UDocTableProduct{manager: f.relations, doc: sieve, table: table}, nil)
	raw, err = source.Rename([]uint32{0, 1})
	wrongLayout := ownUDocTableProduct(t, raw, err)
	for _, invalid := range []*UDocTableProduct{wrongMask, wrongLayout} {
		if err := target.UnionFrom(invalid, nil); err == nil {
			t.Fatal("unaligned source accepted by concrete aligned union")
		}
		if err := target.UnionFrom(source, invalid); err == nil {
			t.Fatal("unaligned delta accepted by concrete aligned union")
		}
		assertUDocTableProductFormula(t, f.ast, target, old)
		assertUDocTableProductFormula(t, f.ast, source, added)
	}
	if err := target.AddFact([]ast.ASTHandle{values[3], values[2], values[1]}); err == nil {
		t.Fatal("wrong-sort fact accepted at ignored Bool column")
	}
	assertUDocTableProductFormula(t, f.ast, target, old)
	assertUDocSieveContains(t, target.Doc(), false, values[3], f.truth, values[1])
}

func TestUDocTableProductRenameThreeCyclePreservesOriginalCoordinates(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	source := udocTableProductOperationShape(t, f, doc)
	row := []ast.ASTHandle{values[1], f.truth, values[2]}
	astTestMust(t, source.AddFact(row))
	raw, err := source.Rename([]uint32{0, 1, 2})
	renamed := ownUDocTableProduct(t, raw, err)
	sorts := renamed.Sorts()
	if len(sorts) != 3 || sorts[0] != f.boolean || sorts[1] != f.bits || sorts[2] != f.bits {
		t.Fatal("three-cycle moved original sorts in the wrong direction", sorts)
	}
	assertUDocSieveMask(t, renamed.Doc(), false, true, true)
	rotated := []ast.ASTHandle{f.truth, values[2], values[1]}
	assertUDocTableProductContains(t, renamed, true, rotated...)
	assertUDocTableProductContains(t, renamed, false, f.truth, values[1], values[2])
	assertUDocTableProductFormula(t, f.ast, renamed, rotated)
	assertUDocTableProductFormula(t, f.ast, source, row)
	raw, err = renamed.Rename([]uint32{0, 2, 1})
	restored := ownUDocTableProduct(t, raw, err)
	assertUDocSieveMask(t, restored.Doc(), true, false, true)
	assertUDocTableProductFormula(t, f.ast, restored, row)
	astTestMust(t, source.Close())
	astTestMust(t, renamed.Close())
	assertUDocTableProductContains(t, restored, true, row...)
}

func TestUDocTableProductFiltersPreserveDOCIgnoredColumnsAndCloneIndependence(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	source := udocTableProductOperationShape(t, f, doc)
	first := []ast.ASTHandle{values[1], f.truth, values[1]}
	unequal := []ast.ASTHandle{values[1], f.falsehood, values[2]}
	second := []ast.ASTHandle{values[2], f.falsehood, values[2]}
	for _, row := range [][]ast.ASTHandle{first, unequal, second} {
		astTestMust(t, source.AddFact(row))
	}
	raw, err := source.Clone()
	boolean := ownUDocTableProduct(t, raw, err)
	astTestMust(t, boolean.FilterEqual(1, f.truth))
	assertUDocTableProductFormula(t, f.ast, boolean, first)
	assertUDocSieveContains(t, boolean.Doc(), true, second...)
	assertUDocSieveContains(t, boolean.Doc(), true, unequal...)
	raw, err = source.Clone()
	identical := ownUDocTableProduct(t, raw, err)
	astTestMust(t, identical.FilterIdentical([]uint32{0, 2}))
	assertUDocTableProductFormula(t, f.ast, identical, first, second)
	assertUDocSieveContains(t, identical.Doc(), false, unequal...)
	astTestMust(t, identical.FilterEqual(0, values[2]))
	assertUDocTableProductFormula(t, f.ast, identical, second)
	assertUDocTableProductFormula(t, f.ast, source, first, unequal, second)
	if err := source.FilterEqual(1, values[1]); err == nil {
		t.Fatal("wrong-sort Bool filter accepted")
	}
	if err := source.FilterIdentical([]uint32{0, 1}); err == nil {
		t.Fatal("mixed-sort identical filter accepted")
	}
	assertUDocTableProductFormula(t, f.ast, source, first, unequal, second)
}

func TestUDocTableProductBooleanIdentityFilterRunsOnFiniteComponent(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	finite := udocTableProductFinite(t, []uint64{2, 2})
	raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean, f.boolean}, finite, nil, nil)
	product := ownUDocTableProduct(t, raw, err)
	equal := []ast.ASTHandle{values[1], f.truth, f.truth}
	unequal := []ast.ASTHandle{values[2], f.truth, f.falsehood}
	astTestMust(t, product.AddFact(equal))
	astTestMust(t, product.AddFact(unequal))
	astTestMust(t, product.FilterIdentical([]uint32{1, 2}))
	assertUDocTableProductFormula(t, f.ast, product, equal)
	assertUDocSieveContains(t, product.Doc(), true, unequal...)
}
