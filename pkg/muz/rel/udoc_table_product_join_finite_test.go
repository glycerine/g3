package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocTableProductJoinsRawFiniteInputInBothOrientations(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, finiteFirst := range []bool{false, true} {
			t.Run(kind+"/finite-first="+map[bool]string{false: "false", true: "true"}[finiteFirst], func(t *testing.T) {
				f, factory, _, values := rawUDocProductJoinFixture(t, kind)
				productSorts := []ast.ASTHandle{f.boolean, f.bits}
				finiteSorts := []ast.ASTHandle{f.bits, f.boolean}
				doc, err := NewEmptyUDocSieve(factory, productSorts)
				astTestMust(t, err)
				for _, row := range [][]ast.ASTHandle{{f.truth, values[1]}, {f.falsehood, values[2]}} {
					if _, err = doc.AddFact(row); err != nil {
						t.Fatal(err)
					}
				}
				productTable := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1}, Fact{0, 2})
				product := ownUDocTableProduct(t, &UDocTableProduct{manager: f.relations, doc: doc, table: productTable}, nil)
				finite := rawUDocProductJoinFinite(t, f, []uint64{4, 2}, Fact{1, 1})
				var joined *UDocTableProduct
				if finiteFirst {
					joined, err = JoinFiniteProduct(f.relations, finiteSorts, finite, product, []uint32{0}, []uint32{1})
				} else {
					joined, err = JoinProductFinite(product, finiteSorts, finite, []uint32{1}, []uint32{0})
				}
				joined = ownUDocTableProduct(t, joined, err)
				wantSorts := []ast.ASTHandle{f.boolean, f.bits, f.bits, f.boolean}
				wantTuple := []ast.ASTHandle{f.truth, values[1], values[1], f.truth}
				wrongTuple := []ast.ASTHandle{f.truth, values[1], values[1], f.falsehood}
				if finiteFirst {
					wantSorts = []ast.ASTHandle{f.bits, f.boolean, f.boolean, f.bits}
					wantTuple = []ast.ASTHandle{values[1], f.truth, f.truth, values[1]}
					wrongTuple = []ast.ASTHandle{values[1], f.truth, f.falsehood, values[1]}
				}
				gotSorts := joined.Sorts()
				if len(gotSorts) != len(wantSorts) {
					t.Fatal("raw finite/product join changed operand order", gotSorts, wantSorts)
				}
				for i := range wantSorts {
					if gotSorts[i] != wantSorts[i] {
						t.Fatal("raw finite/product join changed operand order", gotSorts, wantSorts)
					}
				}
				assertUDocTableProductContains(t, joined, true, wantTuple...)
				assertUDocTableProductContains(t, joined, false, wrongTuple...)
				if joined.Doc().Inner() == doc.Inner() || joined.Table() == productTable || joined.Table() == finite {
					t.Fatal("raw finite/product join aliased a borrowed component")
				}
				if got := rawUDocProductFiniteKind(joined.Table()); got != rawUDocProductFiniteKind(productTable) {
					t.Fatal("raw finite/product join changed concrete table kind", got)
				}
				if kind == "lazy_sparse" && !f.relations.lazyFinite.Owns(joined.Table()) {
					t.Fatal("raw finite/product join changed lazy factory")
				}
				if found, err := doc.ContainsFact([]ast.ASTHandle{f.truth, values[1]}); err != nil || !found {
					t.Fatal("raw finite/product join changed its DOC input", found, err)
				}
				if found, err := finite.Contains(Fact{1, 1}); err != nil || !found {
					t.Fatal("raw finite/product join changed its input", found, err)
				}
			})
		}
	}
}
