package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocTableProductCloneForManagerOwnsQueryComponents(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f, factory, doc, values := udocTableProductFixture(t)
			checkedUDocAdd(t, doc, values[1])
			second, err := NewRelationManager(f.ast)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, second.Close()) })
			var sourceLazy, targetLazy *LazySparseFiniteFactory
			if kind == "lazy_sparse" {
				sourceLazy, targetLazy = NewLazySparseFiniteFactory(), NewLazySparseFiniteFactory()
				t.Cleanup(func() { astTestMust(t, sourceLazy.Close()); astTestMust(t, targetLazy.Close()) })
				astTestMust(t, f.relations.SetLazySparseFiniteFactory(sourceLazy))
				astTestMust(t, second.SetLazySparseFiniteFactory(targetLazy))
			}
			astTestMust(t, f.relations.SetDefaultFiniteTable(kind))
			astTestMust(t, second.SetDefaultFiniteTable(kind))
			signature, err := NewSignature([]uint64{2})
			astTestMust(t, err)
			finite, err := f.relations.NewDefaultFiniteTable(signature)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, finite.Close()) })
			_, err = finite.Add(Fact{1})
			astTestMust(t, err)
			raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean}, finite, nil, nil)
			original := ownUDocTableProduct(t, raw, err)
			raw, err = original.CloneForManager(second)
			copy := ownUDocTableProduct(t, raw, err)
			if copy.manager != second || copy.Doc().Inner().factory != factory || copy.Doc() == original.Doc() || copy.Table() == original.Table() {
				t.Fatal("query clone changed borrowed DOC factory or aliased component storage")
			}
			if kind == "lazy_sparse" {
				if !targetLazy.Owns(copy.Table()) || sourceLazy.Owns(copy.Table()) || !sourceLazy.Owns(original.Table()) {
					t.Fatal("query clone did not migrate actual lazy engine ownership")
				}
			}
			assertUDocTableProductContains(t, copy, true, values[1], f.truth)
			otherDoc, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, otherDoc.Close()) })
			checkedUDocAdd(t, otherDoc, values[2])
			otherFinite, err := second.NewDefaultFiniteTable(signature)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, otherFinite.Close()) })
			_, err = otherFinite.Add(Fact{0})
			astTestMust(t, err)
			raw, err = JoinUDocFinite(second, otherDoc, []ast.ASTHandle{f.boolean}, otherFinite, nil, nil)
			fresh := ownUDocTableProduct(t, raw, err)
			raw, err = copy.EmptyLike()
			delta := ownUDocTableProduct(t, raw, err)
			astTestMust(t, copy.UnionFrom(fresh, delta))
			assertUDocTableProductContains(t, copy, true, values[2], f.falsehood)
			assertUDocTableProductContains(t, delta, true, values[2], f.falsehood)
			assertUDocTableProductContains(t, original, false, values[2], f.falsehood)
			assertUDocTableProductContains(t, original, true, values[1], f.truth)
			astTestMust(t, copy.Close())
			assertUDocTableProductContains(t, original, true, values[1], f.truth)
			// Projected nullary lazy owners must also bind to the destination
			// engine rather than collapsing into an unrelated Hash component.
			raw, err = original.Project([]uint32{0, 1})
			nullary := ownUDocTableProduct(t, raw, err)
			raw, err = nullary.CloneForManager(second)
			nullaryCopy := ownUDocTableProduct(t, raw, err)
			assertUDocTableProductContains(t, nullaryCopy, true)
			if kind == "lazy_sparse" && !targetLazy.Owns(nullaryCopy.Table()) {
				t.Fatal("projected query clone lost concrete lazy owner")
			}
		})
	}
}

func TestUDocTableProductCloneForManagerRejectsForeignAndClosedCodecs(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	checkedUDocAdd(t, doc, values[1])
	finite := udocTableProductFinite(t, []uint64{2}, Fact{1})
	raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean}, finite, nil, nil)
	original := ownUDocTableProduct(t, raw, err)
	foreign := newTableRelationFixture(t)
	closed, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	for _, manager := range []*RelationManager{nil, foreign.relations, closed} {
		copy, err := original.CloneForManager(manager)
		if err == nil || copy != nil {
			if copy != nil {
				_ = copy.Close()
			}
			t.Fatal("invalid query codec published cloned product", copy, err)
		}
		assertUDocTableProductContains(t, original, true, values[1], f.truth)
	}
}
