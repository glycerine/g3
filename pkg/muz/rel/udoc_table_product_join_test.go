package rel

// Source: dl_product_relation.cpp join_fn matches each existing component;
// dl_relation_manager.cpp selects the first capable plugin for mixed tables.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func udocProductPairInput(t *testing.T, f *tableRelationFixture, factory *UDocRelationFactory, values []ast.ASTHandle, kind string, bitsFirst bool, keys []int, facts ...Fact) *UDocTableProduct {
	t.Helper()
	astTestMust(t, f.relations.SetDefaultFiniteTable(kind))
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	domains := []uint64{2, 4}
	if bitsFirst {
		sorts = []ast.ASTHandle{f.bits, f.boolean}
		domains = []uint64{4, 2}
	}
	sieve, err := NewEmptyUDocSieve(factory, sorts)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, sieve.Close()) })
	for _, key := range keys {
		row := []ast.ASTHandle{f.falsehood, values[key]}
		if bitsFirst {
			row = []ast.ASTHandle{values[key], f.falsehood}
		}
		_, err := sieve.AddFact(row)
		astTestMust(t, err)
	}
	table := rawUDocProductJoinFinite(t, f, domains, facts...)
	return ownUDocTableProduct(t, &UDocTableProduct{manager: f.relations, doc: sieve, table: table}, nil)
}

func TestUDocTableProductPairJoinMatchesBothComponents(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			for _, key := range []string{"cartesian", "bv", "bool", "both"} {
				t.Run(fmt.Sprintf("%s/reverse=%v/%s", kind, reverse, key), func(t *testing.T) {
					f, factory, _, values := rawUDocProductJoinFixture(t, kind)
					left := udocProductPairInput(t, f, factory, values, kind, true, []int{1, 2}, Fact{1, 1}, Fact{2, 0}, Fact{3, 1})
					right := udocProductPairInput(t, f, factory, values, kind, false, []int{1, 3}, Fact{0, 1}, Fact{1, 2}, Fact{1, 3})
					var lc, rc []uint32
					joinBV, joinBool := key == "bv" || key == "both", key == "bool" || key == "both"
					if joinBV {
						lc, rc = append(lc, 0), append(rc, 1)
					}
					if joinBool {
						lc, rc = append(lc, 1), append(rc, 0)
					}
					var raw *UDocTableProduct
					var err error
					if reverse {
						raw, err = right.Join(left, rc, lc)
					} else {
						raw, err = left.Join(right, lc, rc)
					}
					joined := ownUDocTableProduct(t, raw, err)
					mask := []bool{true, false, false, true}
					if reverse {
						mask = []bool{false, true, true, false}
					}
					assertUDocSieveMask(t, joined.Doc(), mask...)
					if joined.Table() == left.Table() || joined.Table() == right.Table() || joined.Doc().Inner() == left.Doc().Inner() || joined.Doc().Inner() == right.Doc().Inner() {
						t.Fatal("product join aliased borrowed component owner")
					}
					if rawUDocProductFiniteKind(joined.Table()) != rawUDocProductFiniteKind(left.Table()) {
						t.Fatal("same-kind product join changed concrete table")
					}
					// Lazy results must own their operands before the first table
					// query forces evaluation, including when both inputs close.
					astTestMust(t, left.Close())
					astTestMust(t, right.Close())
					var rows [][]ast.ASTHandle
					for x, xv := range values {
						for b, bv := range []ast.ASTHandle{f.falsehood, f.truth} {
							for c, cv := range []ast.ASTHandle{f.falsehood, f.truth} {
								for y, yv := range values {
									wantDoc := (x == 1 || x == 2) && (y == 1 || y == 3) && (!joinBV || x == y)
									wantTable := (b == 1 && (x == 1 || x == 3) || b == 0 && x == 2) && (c == 0 && y == 1 || c == 1 && (y == 2 || y == 3)) && (!joinBV || x == y) && (!joinBool || b == c)
									row := []ast.ASTHandle{xv, bv, cv, yv}
									fact := Fact{Element(x), Element(b), Element(c), Element(y)}
									if reverse {
										row = []ast.ASTHandle{cv, yv, xv, bv}
										fact = Fact{Element(c), Element(y), Element(x), Element(b)}
									}
									assertUDocSieveContains(t, joined.Doc(), wantDoc, row...)
									found, err := joined.Table().Contains(fact)
									astTestMust(t, err)
									if found != wantTable {
										t.Fatal("product join changed the independent finite intersection", fact, found, wantTable)
									}
									assertUDocTableProductContains(t, joined, wantDoc && wantTable, row...)
									if wantDoc && wantTable {
										rows = append(rows, row)
									}
								}
							}
						}
					}
					assertUDocTableProductFormula(t, f.ast, joined, rows...)
				})
			}
		}
	}
}

func TestUDocTableProductPairJoinDistinguishesComponentContradictions(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, tc := range []struct {
			name                 string
			docKey, tableKey     int
			docEmpty, tableEmpty bool
		}{
			{"coarse-false-formula", 1, 2, false, false},
			{"doc-empty", 3, 2, true, false},
			{"table-empty", 1, 3, false, true},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				f, factory, _, values := rawUDocProductJoinFixture(t, kind)
				left := udocProductPairInput(t, f, factory, values, kind, true, []int{1}, Fact{2, 1})
				right := udocProductPairInput(t, f, factory, values, kind, false, []int{tc.docKey}, Fact{0, Element(tc.tableKey)})
				raw, err := left.Join(right, []uint32{0}, []uint32{1})
				joined := ownUDocTableProduct(t, raw, err)
				if empty, err := joined.Doc().Empty(); err != nil || empty != tc.docEmpty {
					t.Fatal("product join substituted a full DOC component", empty, tc.docEmpty, err)
				}
				if count, err := joined.Table().Len(); err != nil || (count == 0) != tc.tableEmpty {
					t.Fatal("product join substituted a full finite component", count, tc.tableEmpty, err)
				}
				if empty, err := joined.Empty(); err != nil || empty != (tc.docEmpty || tc.tableEmpty) {
					t.Fatal("product join replaced source coarse emptiness", empty, err)
				}
				assertUDocTableProductFormula(t, f.ast, joined)
				assertUDocTableProductFormula(t, f.ast, left)
			})
		}
	}
}

func TestUDocTableProductPairSelfJoinOwnsResult(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f, factory, _, values := rawUDocProductJoinFixture(t, kind)
			input := udocProductPairInput(t, f, factory, values, kind, true, []int{1}, Fact{1, 0}, Fact{1, 1})
			raw, err := input.Join(input, []uint32{0}, []uint32{0})
			joined := ownUDocTableProduct(t, raw, err)
			assertUDocTableProductFormula(t, f.ast, input, []ast.ASTHandle{values[1], f.falsehood}, []ast.ASTHandle{values[1], f.truth})
			astTestMust(t, input.Close())
			var rows [][]ast.ASTHandle
			for _, first := range []ast.ASTHandle{f.falsehood, f.truth} {
				for _, second := range []ast.ASTHandle{f.falsehood, f.truth} {
					rows = append(rows, []ast.ASTHandle{values[1], first, values[1], second})
				}
			}
			assertUDocTableProductFormula(t, f.ast, joined, rows...)
		})
	}
}

func TestUDocTableProductPairJoinMixedFiniteKindsFollowSourceFallback(t *testing.T) {
	for _, leftKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, rightKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
			if leftKind == rightKind {
				continue
			}
			t.Run(leftKind+"/"+rightKind, func(t *testing.T) {
				f, factory, _, values := rawUDocProductJoinFixture(t, "lazy_sparse")
				left := udocProductPairInput(t, f, factory, values, leftKind, true, []int{1}, Fact{1, 1})
				right := udocProductPairInput(t, f, factory, values, rightKind, false, []int{1}, Fact{0, 1})
				wantKind := rawUDocProductFiniteKind(left.Table())
				raw, err := left.Join(right, []uint32{0}, []uint32{1})
				joined := ownUDocTableProduct(t, raw, err)
				// All these ordinary nonnull signatures are supported by the
				// left plugin, so the source fallback chooses that concrete kind.
				if rawUDocProductFiniteKind(joined.Table()) != wantKind {
					t.Fatalf("mixed join selected %T instead of left plugin %s", joined.Table(), leftKind)
				}
				astTestMust(t, left.Close())
				astTestMust(t, right.Close())
				assertUDocTableProductFormula(t, f.ast, joined, []ast.ASTHandle{values[1], f.truth, f.falsehood, values[1]})
			})
		}
	}
}

func TestUDocTableProductPairJoinIndependentProjectionsAndNullaryFallback(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f, factory, _, values := rawUDocProductJoinFixture(t, kind)
			left := udocProductPairInput(t, f, factory, values, kind, true, []int{1}, Fact{1, 1})
			right := udocProductPairInput(t, f, factory, values, kind, false, []int{2}, Fact{0, 2})
			raw, err := left.Project([]uint32{1})
			bits := ownUDocTableProduct(t, raw, err)
			raw, err = right.Project([]uint32{1})
			booleans := ownUDocTableProduct(t, raw, err)
			raw, err = bits.Join(booleans, nil, nil)
			joined := ownUDocTableProduct(t, raw, err)
			assertUDocSieveMask(t, joined.Doc(), true, false)
			assertUDocTableProductFormula(t, f.ast, joined, []ast.ASTHandle{values[1], f.falsehood})
			raw, err = left.Project([]uint32{0, 1})
			nullary := ownUDocTableProduct(t, raw, err)
			if kind == "sparse" {
				if _, ok := nullary.Table().(*HashTable); !ok {
					t.Fatal("source Sparse nullary projection did not select Hash")
				}
			}
			raw, err = nullary.Join(right, nil, nil)
			fromNullary := ownUDocTableProduct(t, raw, err)
			wantKind := rawUDocProductFiniteKind(nullary.Table())
			if rawUDocProductFiniteKind(fromNullary.Table()) != wantKind {
				t.Fatal("nullary/non-null join changed source first capable plugin")
			}
			astTestMust(t, nullary.Close())
			astTestMust(t, right.Close())
			assertUDocTableProductFormula(t, f.ast, fromNullary, []ast.ASTHandle{f.falsehood, values[2]})
		})
	}
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed-nullary/reverse=%v", reverse), func(t *testing.T) {
			f, factory, _, values := rawUDocProductJoinFixture(t, "lazy_sparse")
			lazySource := udocProductPairInput(t, f, factory, values, "lazy_sparse", true, []int{1}, Fact{1, 1})
			hashSource := udocProductPairInput(t, f, factory, values, "hashtable", false, []int{2}, Fact{0, 2})
			raw, err := lazySource.Project([]uint32{0, 1})
			lazy := ownUDocTableProduct(t, raw, err)
			raw, err = hashSource.Project([]uint32{0, 1})
			hash := ownUDocTableProduct(t, raw, err)
			if reverse {
				raw, err = hash.Join(lazy, nil, nil)
			} else {
				raw, err = lazy.Join(hash, nil, nil)
			}
			joined := ownUDocTableProduct(t, raw, err)
			// Lazy delegates capability to Sparse, which rejects nullary
			// signatures. Mixed fallback therefore selects Hash in both orders.
			if _, ok := joined.Table().(*HashTable); !ok {
				t.Fatalf("mixed nullary join bypassed source plugin capability: %T", joined.Table())
			}
			astTestMust(t, lazy.Close())
			astTestMust(t, hash.Close())
			assertUDocSieveMask(t, joined.Doc())
			assertUDocTableProductFormula(t, f.ast, joined, nil)
		})
	}
}
