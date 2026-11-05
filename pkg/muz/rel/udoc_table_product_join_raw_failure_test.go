package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocTableProductRawJoinAllocationKeepsBorrowedSources(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "doc-first"
		if reverse {
			name = "product-first"
		}
		t.Run(name, func(t *testing.T) {
			failures, completed := 0, false
			for allowance := int64(0); allowance < 1024 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					// Each ordinal starts with a fresh native budget and factory.
					// Small DOC pages exercise core construction before the 512
					// original sort pins cross AST reference allocation pages.
					f := newTableRelationFixture(t)
					config := arena.VectorConfig{PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: f.ast.Budget()}}
					factory, err := NewUDocRelationFactory(f.ast, config)
					astTestMust(t, err)
					t.Cleanup(factory.Close)
					one, two := f.bv(1), f.bv(2)
					astTestPin(t, f.ast, one)
					astTestPin(t, f.ast, two)
					raw, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
					inputDoc := ownUDocTableUnionRelation(t, raw, err)
					checkedUDocAdd(t, inputDoc, one)
					raw, err = factory.NewEmpty([]ast.ASTHandle{f.bits})
					doc := ownUDocTableUnionRelation(t, raw, err)
					checkedUDocAdd(t, doc, two)
					sorts, domains, fact := make([]ast.ASTHandle, 512), make([]uint64, 512), make(Fact, 512)
					inputRow := make([]ast.ASTHandle, 513)
					inputRow[0] = one
					for i := range sorts {
						sorts[i], domains[i], inputRow[i+1] = f.boolean, 2, f.falsehood
					}
					finite := udocTableProductFinite(t, domains, fact)
					built, err := JoinUDocFinite(f.relations, inputDoc, sorts, finite, nil, nil)
					input := ownUDocTableProduct(t, built, err)
					assertUDocTableProductContains(t, input, true, inputRow...)
					before := f.ast.Stats()
					join := func() (*UDocTableProduct, error) {
						if reverse {
							return JoinProductUDoc(input, doc, nil, nil)
						}
						return JoinUDocProduct(doc, input, nil, nil)
					}
					f.ast.Budget().FailAfter(allowance)
					joined, cause := join()
					f.ast.Budget().FailAfter(-1)
					cleanupUDocProductFailure(t, cause)
					if cause != nil {
						failures++
						if joined != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
							if joined != nil {
								astTestMust(t, joined.Close())
							}
							t.Fatal("failed raw/product join published an owner or changed cause", cause)
						}
					} else {
						completed = true
						astTestMust(t, joined.Close())
					}
					after := f.ast.Stats()
					if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
						t.Fatal("raw/product join leaked AST ownership", before, after, cause)
					}
					// A failure must leave both original owners valid for an
					// immediate successful retry, not just for destruction.
					joined, err = join()
					astTestMust(t, err)
					row := append([]ast.ASTHandle{two}, inputRow...)
					if reverse {
						row = append(append([]ast.ASTHandle(nil), inputRow...), two)
					}
					assertUDocTableProductContains(t, joined, true, row...)
					if joined.Doc().Inner() == doc || joined.Doc().Inner() == input.Doc().Inner() || joined.Table() == input.Table() {
						t.Fatal("retry aliased a borrowed component")
					}
					astTestMust(t, joined.Close())
					after = f.ast.Stats()
					if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
						t.Fatal("successful retry leaked AST ownership", before, after)
					}
					assertUDocTableProductContains(t, input, true, inputRow...)
					inputRow[0] = two
					assertUDocTableProductContains(t, input, false, inputRow...)
					checkedUDocContains(t, doc, true, two)
					checkedUDocContains(t, doc, false, one)
					checkedUDocContains(t, inputDoc, true, one)
					checkedUDocContains(t, inputDoc, false, two)
					if found, err := finite.Contains(fact); err != nil || !found {
						t.Fatal("join consumed the original finite source", found, err)
					}
					// docASTManager verifies the complete native budget is zero
					// after all relations, cached DOC managers, and ASTs close.
				}) {
					return
				}
			}
			if failures == 0 || !completed {
				t.Fatal("dense raw/product join campaign missed failure or completion", failures, completed)
			}
		})
	}
}

func TestUDocTableProductRawJoinRejectsIncompatibleSources(t *testing.T) {
	f, factory, doc, values := rawUDocProductJoinFixture(t, "hashtable")
	checkedUDocAdd(t, doc, values[1])
	finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
	input := rawUDocProductJoinInput(t, f, doc, finite)
	foreignFactory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(foreignFactory.Close)
	raw, err := foreignFactory.NewFull([]ast.ASTHandle{f.bits})
	foreign := ownUDocTableUnionRelation(t, raw, err)
	checkedFactory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	t.Cleanup(checkedFactory.Close)
	raw, err = checkedFactory.NewEmpty([]ast.ASTHandle{f.bits})
	checked := ownUDocTableUnionRelation(t, raw, err)
	raw, err = factory.NewFull([]ast.ASTHandle{f.bits, f.bits})
	multi := ownUDocTableUnionRelation(t, raw, err)
	cases := []struct {
		name         string
		doc          *TypedUDocRelation
		left, right  []uint32
		otherDefault bool
	}{
		{name: "nil"},
		{name: "foreign-factory", doc: foreign},
		{name: "checked", doc: checked},
		{name: "multi-column", doc: multi},
		{name: "unequal-key-count", doc: doc, left: []uint32{0}},
		{name: "raw-column-out-of-range", doc: doc, left: []uint32{1}, right: []uint32{1}},
		{name: "product-column-out-of-range", doc: doc, left: []uint32{0}, right: []uint32{2}},
		{name: "different-key-sorts", doc: doc, left: []uint32{0}, right: []uint32{0}},
		{name: "different-default-kind", doc: doc, otherDefault: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.otherDefault {
				astTestMust(t, f.relations.SetDefaultFiniteTable("sparse"))
				defer func() { astTestMust(t, f.relations.SetDefaultFiniteTable("hashtable")) }()
			}
			before := f.ast.Stats()
			for _, reverse := range []bool{false, true} {
				var result *UDocTableProduct
				var cause error
				if reverse {
					result, cause = JoinProductUDoc(input, test.doc, test.right, test.left)
				} else {
					result, cause = JoinUDocProduct(test.doc, input, test.left, test.right)
				}
				cleanupUDocProductFailure(t, cause)
				if result != nil || cause == nil {
					if result != nil {
						astTestMust(t, result.Close())
					}
					t.Fatal("raw/product join accepted incompatible source", reverse, cause)
				}
				after := f.ast.Stats()
				if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
					t.Fatal("rejected join leaked AST ownership", before, after)
				}
				assertUDocTableProductContains(t, input, true, f.truth, values[1])
				assertUDocTableProductContains(t, input, false, f.truth, values[2])
				checkedUDocContains(t, doc, true, values[1])
				checkedUDocContains(t, doc, false, values[2])
			}
		})
	}
}

func TestUDocTableProductWideSortUsesSieveMask(t *testing.T) {
	f, factory, doc, values := rawUDocProductJoinFixture(t, "hashtable")
	wideSort, err := f.ast.BitVectorSort(96)
	astTestMust(t, err)
	astTestPin(t, f.ast, wideSort)
	raw, err := factory.NewFull([]ast.ASTHandle{wideSort})
	wideDoc := ownUDocTableUnionRelation(t, raw, err)
	finite := rawUDocProductJoinFinite(t, f, []uint64{2}, Fact{1})

	product, err := JoinUDocFinite(f.relations, wideDoc, []ast.ASTHandle{f.boolean}, finite, nil, nil)
	product = ownUDocTableProduct(t, product, err)
	if got := product.TableColumns(); len(got) != 2 || got[0] || !got[1] {
		want := []bool{false, true}
		t.Fatal("wide sort was not represented by a sieve column", got, want)
	}
	if count, err := product.Table().Len(); err != nil || count != 1 {
		t.Fatal("ignored wide column changed finite table rows", count, err)
	}
	if found, err := product.Table().Contains(Fact{1}); err != nil || !found {
		t.Fatal("wide-sort sieve join lost finite row", found, err)
	}
	if empty, err := product.Empty(); err != nil || empty {
		t.Fatal("wide-sort sieve join became empty", empty, err)
	}
	astTestMust(t, product.Close())
	astTestMust(t, wideDoc.Close())

	// A raw wide-sort component joined with an existing DOC/table product
	// keeps the same ignored-column mask in the combined table abstraction.
	checkedUDocAdd(t, doc, values[1])
	rawFinite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
	input := rawUDocProductJoinInput(t, f, doc, rawFinite)
	rawWide, err := factory.NewFull([]ast.ASTHandle{wideSort})
	wide := ownUDocTableUnionRelation(t, rawWide, err)
	joined, err := JoinUDocProduct(wide, input, nil, nil)
	joined = ownUDocTableProduct(t, joined, err)
	if got := joined.TableColumns(); len(got) != 3 || got[0] || !got[1] || !got[2] {
		want := []bool{false, true, true}
		t.Fatal("raw/product join lost ignored wide column", got, want)
	}
	astTestMust(t, joined.Close())
	astTestMust(t, input.Close())
	astTestMust(t, doc.Close())
	astTestMust(t, wide.Close())
}

func TestUDocTableProductRawJoinRejectsForeignLazyOwner(t *testing.T) {
	f, _, doc, values := rawUDocProductJoinFixture(t, "lazy_sparse")
	checkedUDocAdd(t, doc, values[1])
	finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
	input := rawUDocProductJoinInput(t, f, doc, finite)
	originalFactory, originalTable := f.relations.lazyFinite, input.Table()
	foreignFactory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { astTestMust(t, foreignFactory.Close()) })
	astTestMust(t, f.relations.SetLazySparseFiniteFactory(foreignFactory))
	before := f.ast.Stats()
	for _, reverse := range []bool{false, true} {
		var result *UDocTableProduct
		var cause error
		if reverse {
			result, cause = JoinProductUDoc(input, doc, []uint32{1}, []uint32{0})
		} else {
			result, cause = JoinUDocProduct(doc, input, []uint32{0}, []uint32{1})
		}
		cleanupUDocProductFailure(t, cause)
		if result != nil || !errors.Is(cause, ErrUDocTableProduct) {
			if result != nil {
				astTestMust(t, result.Close())
			}
			t.Fatal("raw/product join accepted a different lazy owner", reverse, cause)
		}
		after := f.ast.Stats()
		if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
			t.Fatal("foreign lazy rejection leaked AST ownership", before, after)
		}
		if input.Table() != originalTable || !originalFactory.Owns(originalTable) || foreignFactory.Owns(originalTable) {
			t.Fatal("foreign lazy rejection migrated the input table")
		}
		assertUDocTableProductContains(t, input, true, f.truth, values[1])
	}
	astTestMust(t, f.relations.SetLazySparseFiniteFactory(originalFactory))
	for _, reverse := range []bool{false, true} {
		var result *UDocTableProduct
		var err error
		row := []ast.ASTHandle{values[1], f.truth, values[1]}
		if reverse {
			result, err = JoinProductUDoc(input, doc, []uint32{1}, []uint32{0})
			row = []ast.ASTHandle{f.truth, values[1], values[1]}
		} else {
			result, err = JoinUDocProduct(doc, input, []uint32{0}, []uint32{1})
		}
		astTestMust(t, err)
		if !originalFactory.Owns(result.Table()) {
			t.Fatal("retry changed lazy owner")
		}
		assertUDocTableProductContains(t, result, true, row...)
		astTestMust(t, result.Close())
	}
	assertUDocTableProductContains(t, input, true, f.truth, values[1])
}
