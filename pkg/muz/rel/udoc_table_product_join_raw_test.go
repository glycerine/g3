package rel

// Source: dl_product_relation.cpp join_fn matches DOC with the existing DOC
// sieve and synthesizes only the absent finite component over the raw input.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func rawUDocProductJoinFixture(t *testing.T, kind string) (*tableRelationFixture, *UDocRelationFactory, *TypedUDocRelation, []ast.ASTHandle) {
	t.Helper()
	f, factory, doc, values := udocTableProductFixture(t)
	if kind == "lazy_sparse" {
		lazy := NewLazySparseFiniteFactory()
		t.Cleanup(func() { astTestMust(t, lazy.Close()) })
		astTestMust(t, f.relations.SetLazySparseFiniteFactory(lazy))
	}
	astTestMust(t, f.relations.SetDefaultFiniteTable(kind))
	return f, factory, doc, values
}

func rawUDocProductJoinFinite(t *testing.T, f *tableRelationFixture, domains []uint64, facts ...Fact) FiniteTable {
	t.Helper()
	signature, err := NewSignature(domains)
	astTestMust(t, err)
	finite, err := f.relations.NewDefaultFiniteTable(signature)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, finite.Close()) })
	for _, fact := range facts {
		_, err := finite.Add(fact)
		astTestMust(t, err)
	}
	return finite
}

// This builds P(Bool,BV) with independently chosen DOC and table components.
// The keyed join preserves both key copies until projection removes the raw
// copy, so DOC and finite contradictions remain visible to the source oracle.
func rawUDocProductJoinInput(t *testing.T, f *tableRelationFixture, doc *TypedUDocRelation, finite FiniteTable) *UDocTableProduct {
	t.Helper()
	raw, err := JoinFiniteUDoc(f.relations, []ast.ASTHandle{f.boolean, f.bits}, finite, doc, []uint32{1}, []uint32{0})
	joined := ownUDocTableProduct(t, raw, err)
	raw, err = joined.Project([]uint32{2})
	return ownUDocTableProduct(t, raw, err)
}

func TestUDocTableProductRawJoinMatchesExistingComponents(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			for _, keyed := range []bool{false, true} {
				name := kind + "/doc-first"
				if reverse {
					name = kind + "/product-first"
				}
				if keyed {
					name += "/keyed"
				} else {
					name += "/cartesian"
				}
				t.Run(name, func(t *testing.T) {
					f, factory, doc, values := rawUDocProductJoinFixture(t, kind)
					checkedUDocAdd(t, doc, values[1])
					checkedUDocAdd(t, doc, values[3])
					rawDoc, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
					inputDoc := ownUDocTableUnionRelation(t, rawDoc, err)
					checkedUDocAdd(t, inputDoc, values[1])
					checkedUDocAdd(t, inputDoc, values[2])
					finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1}, Fact{0, 2}, Fact{1, 3})
					input := rawUDocProductJoinInput(t, f, inputDoc, finite)
					var left, right []uint32
					if keyed {
						left, right = []uint32{0}, []uint32{1}
					}
					var raw *UDocTableProduct
					if reverse {
						raw, err = JoinProductUDoc(input, doc, right, left)
					} else {
						raw, err = JoinUDocProduct(doc, input, left, right)
					}
					joined := ownUDocTableProduct(t, raw, err)
					mask := []bool{true, false, true}
					if reverse {
						mask = []bool{false, true, true}
					}
					assertUDocSieveMask(t, joined.Doc(), mask...)
					if joined.Doc() == input.Doc() || joined.Doc().Inner() == doc || joined.Doc().Inner() == input.Doc().Inner() || joined.Table() == input.Table() {
						t.Fatal("raw/product join aliased borrowed component storage")
					}
					if rawUDocProductFiniteKind(joined.Table()) != rawUDocProductFiniteKind(finite) {
						t.Fatal("raw/product join changed concrete finite kind")
					}
					if kind == "lazy_sparse" && !f.relations.lazyFinite.Owns(joined.Table()) {
						t.Fatal("raw/product join changed lazy engine")
					}
					var rows [][]ast.ASTHandle
					for d, value := range values {
						for b, boolean := range []ast.ASTHandle{f.falsehood, f.truth} {
							for k, key := range values {
								wantDoc := (d == 1 || d == 3) && (k == 1 || k == 2) && (!keyed || d == k)
								wantTable := (b == 1 && (k == 1 || k == 3) || b == 0 && k == 2) && (!keyed || d == k)
								row := []ast.ASTHandle{value, boolean, key}
								fact := Fact{Element(d), Element(b), Element(k)}
								if reverse {
									row = []ast.ASTHandle{boolean, key, value}
									fact = Fact{Element(b), Element(k), Element(d)}
								}
								assertUDocSieveContains(t, joined.Doc(), wantDoc, row...)
								found, err := joined.Table().Contains(fact)
								astTestMust(t, err)
								if found != wantTable {
									t.Fatal("raw/product join finite component was narrowed by raw DOC", fact, found, wantTable)
								}
								assertUDocTableProductContains(t, joined, wantDoc && wantTable, row...)
								if wantDoc && wantTable {
									rows = append(rows, row)
								}
							}
						}
					}
					assertUDocTableProductFormula(t, f.ast, joined, rows...)
					removed := []uint32{2}
					if reverse {
						removed = []uint32{1}
					}
					raw, err = joined.Project(removed)
					projected := ownUDocTableProduct(t, raw, err)
					if reverse {
						assertUDocSieveMask(t, projected.Doc(), false, true)
						raw, err = projected.Rename([]uint32{0, 1})
						projected = ownUDocTableProduct(t, raw, err)
					}
					assertUDocSieveMask(t, projected.Doc(), true, false)
					var projectedRows [][]ast.ASTHandle
					for d, value := range values {
						for b, boolean := range []ast.ASTHandle{f.falsehood, f.truth} {
							want := (d == 1 || d == 3) && (!keyed || d == 1 && b == 1)
							assertUDocTableProductContains(t, projected, want, value, boolean)
							if want {
								projectedRows = append(projectedRows, []ast.ASTHandle{value, boolean})
							}
						}
					}
					assertUDocTableProductFormula(t, f.ast, input, []ast.ASTHandle{f.truth, values[1]}, []ast.ASTHandle{f.falsehood, values[2]})
					for d, value := range values {
						checkedUDocContains(t, doc, d == 1 || d == 3, value)
					}
					if count, err := finite.Len(); err != nil || count != 3 {
						t.Fatal("raw/product join mutated finite input", count, err)
					}
					astTestMust(t, input.Close())
					astTestMust(t, inputDoc.Close())
					astTestMust(t, doc.Close())
					astTestMust(t, finite.Close())
					assertUDocTableProductFormula(t, f.ast, joined, rows...)
					astTestMust(t, joined.Close())
					assertUDocTableProductFormula(t, f.ast, projected, projectedRows...)
				})
			}
		}
	}
}

func TestUDocTableProductRawJoinPreservesCoarseAndActualEmptiness(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f, factory, inputDoc, values := rawUDocProductJoinFixture(t, kind)
			checkedUDocAdd(t, inputDoc, values[2])
			finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
			input := rawUDocProductJoinInput(t, f, inputDoc, finite)
			if empty, err := input.Empty(); err != nil || empty {
				t.Fatal("oracle input must have two conflicting nonempty components", empty, err)
			}
			assertUDocTableProductFormula(t, f.ast, input)
			for _, value := range []int{1, 2} {
				rawDoc, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
				doc := ownUDocTableUnionRelation(t, rawDoc, err)
				checkedUDocAdd(t, doc, values[value])
				raw, err := JoinUDocProduct(doc, input, []uint32{0}, []uint32{1})
				joined := ownUDocTableProduct(t, raw, err)
				wantEmpty := value == 1
				if empty, err := joined.Empty(); err != nil || empty != wantEmpty {
					t.Fatal("raw/product join changed source component emptiness", value, empty, wantEmpty, err)
				}
				if empty, err := joined.Doc().Empty(); err != nil || empty != wantEmpty {
					t.Fatal("raw/product join ignored the existing DOC constraint", value, empty, err)
				}
				if count, err := joined.Table().Len(); err != nil || count != 1 {
					t.Fatal("raw/product join improperly intersected finite and DOC components", value, count, err)
				}
				assertUDocTableProductFormula(t, f.ast, joined)
				raw, err = joined.Project([]uint32{0, 1, 2})
				nullary := ownUDocTableProduct(t, raw, err)
				if empty, err := nullary.Empty(); err != nil || empty != wantEmpty {
					t.Fatal("nullary projection erased matched DOC emptiness", value, empty, err)
				}
				if wantEmpty {
					assertUDocTableProductFormula(t, f.ast, nullary)
				} else {
					assertUDocTableProductFormula(t, f.ast, nullary, nil)
				}
			}
		})
	}
}

func TestUDocTableProductRawJoinGroundUnionKeepsSourceCorrelationLoss(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f, factory, firstDoc, values := rawUDocProductJoinFixture(t, kind)
			checkedUDocAdd(t, firstDoc, values[1])
			firstTable := rawUDocProductJoinFinite(t, f, []uint64{2}, Fact{1})
			raw, err := JoinFiniteUDoc(f.relations, []ast.ASTHandle{f.boolean}, firstTable, firstDoc, nil, nil)
			input := ownUDocTableProduct(t, raw, err)
			rawDoc, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
			secondDoc := ownUDocTableUnionRelation(t, rawDoc, err)
			checkedUDocAdd(t, secondDoc, values[2])
			secondTable := rawUDocProductJoinFinite(t, f, []uint64{2}, Fact{0})
			raw, err = JoinFiniteUDoc(f.relations, []ast.ASTHandle{f.boolean}, secondTable, secondDoc, nil, nil)
			second := ownUDocTableProduct(t, raw, err)
			// Source ground rules construct separate singleton-column joins.
			// Their componentwise union allows both Bool values at both keys.
			astTestMust(t, input.UnionFrom(second, nil))
			if count, err := input.Table().Len(); err != nil || count != 8 {
				t.Fatal("ground union lost the source full finite key column", count, err)
			}
			assertUDocTableProductFormula(t, f.ast, input,
				[]ast.ASTHandle{f.truth, values[1]}, []ast.ASTHandle{f.falsehood, values[1]},
				[]ast.ASTHandle{f.truth, values[2]}, []ast.ASTHandle{f.falsehood, values[2]})
			raw, err = JoinUDocProduct(firstDoc, input, []uint32{0}, []uint32{1})
			joined := ownUDocTableProduct(t, raw, err)
			raw, err = joined.Project([]uint32{2})
			head := ownUDocTableProduct(t, raw, err)
			assertUDocTableProductContains(t, head, true, values[1], f.truth)
			assertUDocTableProductContains(t, head, true, values[1], f.falsehood)
			assertUDocTableProductContains(t, head, false, values[2], f.truth)
			assertUDocTableProductFormula(t, f.ast, head, []ast.ASTHandle{values[1], f.truth}, []ast.ASTHandle{values[1], f.falsehood})
			assertUDocTableProductFormula(t, f.ast, second, []ast.ASTHandle{f.falsehood, values[2]})
		})
	}
}

func TestUDocTableProductRawJoinProjectedMasksAndFalseInner(t *testing.T) {
	for _, emptyInput := range []bool{false, true} {
		name := "nonempty"
		if emptyInput {
			name = "false-inner"
		}
		t.Run(name, func(t *testing.T) {
			f, factory, inputDoc, values := rawUDocProductJoinFixture(t, "hashtable")
			if !emptyInput {
				checkedUDocAdd(t, inputDoc, values[1])
			}
			finite := rawUDocProductJoinFinite(t, f, []uint64{2}, Fact{1})
			raw, err := JoinUDocFinite(f.relations, inputDoc, []ast.ASTHandle{f.boolean}, finite, nil, nil)
			input := ownUDocTableProduct(t, raw, err)
			rawDoc, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
			doc := ownUDocTableUnionRelation(t, rawDoc, err)
			checkedUDocAdd(t, doc, values[3])
			for _, shape := range []struct {
				name        string
				removed     []uint32
				mask        []bool
				row         []ast.ASTHandle
				finiteCount int
			}{
				{"all-selected-bv", []uint32{1}, []bool{true, true}, []ast.ASTHandle{values[3], values[1]}, 16},
				{"all-ignored-bool", []uint32{0}, []bool{true, false}, []ast.ASTHandle{values[3], f.truth}, 4},
				{"nullary", []uint32{0, 1}, []bool{true}, []ast.ASTHandle{values[3]}, 4},
			} {
				t.Run(shape.name, func(t *testing.T) {
					raw, err := input.Project(shape.removed)
					projected := ownUDocTableProduct(t, raw, err)
					assertUDocSieveMask(t, projected.Doc(), shape.mask[1:]...)
					raw, err = JoinUDocProduct(doc, projected, nil, nil)
					joined := ownUDocTableProduct(t, raw, err)
					assertUDocSieveMask(t, joined.Doc(), shape.mask...)
					if empty, err := joined.Empty(); err != nil || empty != emptyInput {
						t.Fatal("raw/product join reclassified projected DOC mask or false inner", empty, emptyInput, err)
					}
					if count, err := joined.Table().Len(); err != nil || count != shape.finiteCount {
						t.Fatal("projected join lost the full finite raw-DOC column", count, shape.finiteCount, err)
					}
					assertUDocTableProductContains(t, joined, !emptyInput, shape.row...)
					if emptyInput {
						assertUDocTableProductFormula(t, f.ast, joined)
					} else {
						assertUDocTableProductFormula(t, f.ast, joined, shape.row)
					}
					if shape.name == "all-selected-bv" {
						raw, err = JoinUDocProduct(doc, projected, []uint32{0}, []uint32{0})
						keyed := ownUDocTableProduct(t, raw, err)
						if empty, err := keyed.Empty(); err != nil || !empty {
							t.Fatal("projected BV mask lost its selected join equality", empty, err)
						}
						assertUDocTableProductFormula(t, f.ast, keyed)
					}
				})
			}
		})
	}
}
