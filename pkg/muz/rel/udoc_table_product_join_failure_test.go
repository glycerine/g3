package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocTableProductJoinProductsAllocationRetryAndAliases(t *testing.T) {
	for _, self := range []bool{false, true} {
		t.Run(fmt.Sprintf("self=%t", self), func(t *testing.T) {
			failures, completed := 0, false
			for allowance := int64(0); allowance < 1024 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					f := newTableRelationFixture(t)
					factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{
						PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: f.ast.Budget()},
					})
					astTestMust(t, err)
					t.Cleanup(factory.Close)
					one, two := f.bv(1), f.bv(2)
					astTestPin(t, f.ast, one)
					astTestPin(t, f.ast, two)
					build := func(value ast.ASTHandle, count int) (*UDocTableProduct, []ast.ASTHandle) {
						t.Helper()
						raw, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
						doc := ownUDocTableUnionRelation(t, raw, err)
						checkedUDocAdd(t, doc, value)
						sorts, domains, fact := make([]ast.ASTHandle, count), make([]uint64, count), make(Fact, count)
						row := make([]ast.ASTHandle, count+1)
						row[0] = value
						for i := range sorts {
							sorts[i], domains[i], row[i+1] = f.boolean, 2, f.falsehood
						}
						finite := udocTableProductFinite(t, domains, fact)
						product, err := JoinUDocFinite(f.relations, doc, sorts, finite, nil, nil)
						return ownUDocTableProduct(t, product, err), row
					}
					// The factory uses real small native pages, and each join
					// must also pin enough outer sorts to cross ASTRef pages.
					left, leftRow := build(one, 512)
					right, rightRow := left, leftRow
					if !self {
						right, rightRow = build(two, 1)
					}
					before := f.ast.Stats()
					f.ast.Budget().FailAfter(allowance)
					joined, cause := left.Join(right, nil, nil)
					f.ast.Budget().FailAfter(-1)
					cleanupUDocProductFailure(t, cause)
					if cause != nil {
						failures++
						if joined != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
							if joined != nil {
								astTestMust(t, joined.Close())
							}
							t.Fatal("failed product join published an owner or changed cause", cause)
						}
					} else {
						completed = true
						astTestMust(t, joined.Close())
					}
					after := f.ast.Stats()
					if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
						t.Fatal("product join leaked AST ownership", before, after, cause)
					}
					assertUDocTableProductContains(t, left, true, leftRow...)
					assertUDocTableProductContains(t, right, true, rightRow...)
					joined, err = left.Join(right, nil, nil)
					astTestMust(t, err)
					row := append(append([]ast.ASTHandle(nil), leftRow...), rightRow...)
					assertUDocTableProductContains(t, joined, true, row...)
					if joined.Doc().Inner() == left.Doc().Inner() || joined.Doc().Inner() == right.Doc().Inner() || joined.Table() == left.Table() || joined.Table() == right.Table() {
						t.Fatal("successful retry aliased an input component")
					}
					astTestMust(t, joined.Close())
					after = f.ast.Stats()
					if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
						t.Fatal("product retry leaked AST ownership", before, after)
					}
					wrong := append([]ast.ASTHandle(nil), leftRow...)
					wrong[0] = two
					assertUDocTableProductContains(t, left, false, wrong...)
					assertUDocTableProductContains(t, right, true, rightRow...)
					// The fixture checks the entire native budget after closing
					// source products, factory caches, and AST services.
				}) {
					return
				}
			}
			if failures == 0 || !completed {
				t.Fatal("dense product join campaign missed failure or completion", failures, completed)
			}
		})
	}
}

func TestUDocTableProductJoinProductsRejectsIncompatibleOwnersAndKeys(t *testing.T) {
	f, factory, doc, values := rawUDocProductJoinFixture(t, "hashtable")
	checkedUDocAdd(t, doc, values[1])
	finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
	input := rawUDocProductJoinInput(t, f, doc, finite)
	otherManager, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, otherManager.Close()) })
	raw, err := input.CloneForManager(otherManager)
	foreignCodec := ownUDocTableProduct(t, raw, err)
	foreignFactory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(foreignFactory.Close)
	otherDoc, err := foreignFactory.NewEmpty([]ast.ASTHandle{f.bits})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, otherDoc.Close()) })
	checkedUDocAdd(t, otherDoc, values[1])
	foreignDoc := rawUDocProductJoinInput(t, f, otherDoc, finite)
	if foreignDoc.manager != input.manager || foreignDoc.Doc().Inner().factory == factory {
		t.Fatal("foreign DOC fixture did not isolate factory identity")
	}
	cases := []struct {
		name        string
		other       *UDocTableProduct
		left, right []uint32
	}{
		{name: "nil"},
		{name: "same-AST-foreign-codec", other: foreignCodec},
		{name: "same-codec-foreign-DOC-factory", other: foreignDoc},
		{name: "unequal-key-count", other: input, left: []uint32{0}},
		{name: "left-out-of-range", other: input, left: []uint32{2}, right: []uint32{1}},
		{name: "right-out-of-range", other: input, left: []uint32{1}, right: []uint32{2}},
		{name: "different-sort-keys", other: input, left: []uint32{0}, right: []uint32{1}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			before := f.ast.Stats()
			for _, reverse := range []bool{false, true} {
				left, right, lc, rc := input, test.other, test.left, test.right
				if reverse {
					left, right, lc, rc = right, left, rc, lc
				}
				result, cause := left.Join(right, lc, rc)
				cleanupUDocProductFailure(t, cause)
				if result != nil || cause == nil {
					if result != nil {
						astTestMust(t, result.Close())
					}
					t.Fatal("join accepted incompatible products", reverse, cause)
				}
				after := f.ast.Stats()
				if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
					t.Fatal("rejected product join leaked AST ownership", before, after)
				}
				assertUDocTableProductContains(t, input, true, f.truth, values[1])
				assertUDocTableProductContains(t, input, false, f.truth, values[2])
			}
		})
	}
}

func TestUDocTableProductJoinProductsValidatesActualFiniteStorage(t *testing.T) {
	for _, kind := range []string{"wrong-arity", "wrong-domain", "checked", "functional-sparse", "functional-lazy"} {
		t.Run(kind, func(t *testing.T) {
			f, _, doc, values := rawUDocProductJoinFixture(t, "hashtable")
			checkedUDocAdd(t, doc, values[1])
			finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
			input := rawUDocProductJoinInput(t, f, doc, finite)
			domains := []uint64{2, 4}
			if kind == "wrong-arity" {
				domains = []uint64{2}
			} else if kind == "wrong-domain" {
				domains = []uint64{2, 8}
			}
			signature, err := NewSignature(domains)
			astTestMust(t, err)
			var replacement FiniteTable
			switch kind {
			case "checked":
				replacement, err = NewFiniteCheckedPair(input.Table(), "sparse")
			case "functional-sparse":
				replacement, err = NewSparseTable(signature, 1)
			case "functional-lazy":
				lazy := NewLazySparseFiniteFactory()
				t.Cleanup(func() { astTestMust(t, lazy.Close()) })
				astTestMust(t, f.relations.SetLazySparseFiniteFactory(lazy))
				replacement, err = lazy.NewFunctional(signature, 1)
			default:
				replacement = NewHashTable(signature)
			}
			astTestMust(t, err)
			// Use real alternate table owners under an independently cloned
			// outer sieve. The guard must inspect actual storage, not assume
			// it still agrees with the original constructor's signature.
			sieve, err := input.Doc().Clone()
			astTestMust(t, err)
			other := ownUDocTableProduct(t, &UDocTableProduct{manager: f.relations, doc: sieve, table: replacement}, nil)
			before := f.ast.Stats()
			for _, reverse := range []bool{false, true} {
				left, right := input, other
				if reverse {
					left, right = right, left
				}
				result, cause := left.Join(right, nil, nil)
				cleanupUDocProductFailure(t, cause)
				if result != nil || cause == nil {
					if result != nil {
						astTestMust(t, result.Close())
					}
					t.Fatal("join accepted incompatible actual finite storage", reverse, cause)
				}
				after := f.ast.Stats()
				if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
					t.Fatal("storage rejection leaked AST ownership", before, after)
				}
				assertUDocTableProductContains(t, input, true, f.truth, values[1])
				if _, err := replacement.Signature(); err != nil {
					t.Fatal("rejected join consumed borrowed replacement storage", err)
				}
			}
		})
	}
}

func TestUDocTableProductJoinProductsChecksSelectedDOCStorage(t *testing.T) {
	f, _, doc, values := rawUDocProductJoinFixture(t, "hashtable")
	checkedUDocAdd(t, doc, values[1])
	finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
	input := rawUDocProductJoinInput(t, f, doc, finite)
	checkedFactory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	t.Cleanup(checkedFactory.Close)
	checked, err := checkedFactory.NewEmpty([]ast.ASTHandle{f.bits})
	astTestMust(t, err)
	checkedUDocAdd(t, checked, values[1])
	sieve, err := input.Doc().Clone()
	astTestMust(t, err)
	astTestMust(t, sieve.inner.Close())
	sieve.inner = checked
	table, err := input.Table().CloneFinite()
	astTestMust(t, err)
	other := ownUDocTableProduct(t, &UDocTableProduct{manager: f.relations, doc: sieve, table: table}, nil)
	result, cause := other.Join(other, nil, nil)
	joined := ownUDocTableProduct(t, result, cause)
	if !joined.Doc().Inner().FormulaChecked() {
		t.Fatal("product join discarded selected check_relation(doc) wrapper")
	}
	assertUDocTableProductContains(t, joined, true, f.truth, values[1], f.truth, values[1])
	checkedUDocContains(t, checked, true, values[1])
	assertUDocTableProductContains(t, input, true, f.truth, values[1])
}

func TestUDocTableProductJoinProductsRejectsForeignLazyFactoryAndRetries(t *testing.T) {
	f, _, doc, values := rawUDocProductJoinFixture(t, "lazy_sparse")
	checkedUDocAdd(t, doc, values[1])
	finite := rawUDocProductJoinFinite(t, f, []uint64{2, 4}, Fact{1, 1})
	input := rawUDocProductJoinInput(t, f, doc, finite)
	raw, err := input.Clone()
	astTestMust(t, err)
	other := ownUDocTableProduct(t, raw, nil)
	current, original := f.relations.lazyFinite, input.Table()
	foreign := NewLazySparseFiniteFactory()
	t.Cleanup(func() { astTestMust(t, foreign.Close()) })
	astTestMust(t, f.relations.SetLazySparseFiniteFactory(foreign))
	before := f.ast.Stats()
	for _, self := range []bool{false, true} {
		right := other
		if self {
			right = input
		}
		result, cause := input.Join(right, []uint32{1}, []uint32{1})
		cleanupUDocProductFailure(t, cause)
		if result != nil || !errors.Is(cause, ErrUDocTableProduct) {
			if result != nil {
				astTestMust(t, result.Close())
			}
			t.Fatal("join accepted a stale lazy factory", self, cause)
		}
		after := f.ast.Stats()
		if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
			t.Fatal("lazy owner rejection leaked AST ownership", before, after)
		}
		if input.Table() != original || !current.Owns(original) || foreign.Owns(original) {
			t.Fatal("rejected join changed source lazy ownership")
		}
		assertUDocTableProductContains(t, input, true, f.truth, values[1])
		assertUDocTableProductContains(t, other, true, f.truth, values[1])
	}
	astTestMust(t, f.relations.SetLazySparseFiniteFactory(current))
	joined, err := input.Join(other, []uint32{1}, []uint32{1})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, joined.Close()) })
	if !current.Owns(joined.Table()) || joined.Table() == input.Table() || joined.Table() == other.Table() {
		t.Fatal("successful retry changed lazy owner or aliased a source table")
	}
	// The unforced lazy result must own its children independently.
	astTestMust(t, input.Close())
	astTestMust(t, other.Close())
	assertUDocTableProductContains(t, joined, true, f.truth, values[1], f.truth, values[1])
}
