package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func ownUDocTableProduct(t *testing.T, product *UDocTableProduct, err error) *UDocTableProduct {
	t.Helper()
	astTestMust(t, err)
	if product == nil {
		t.Fatal("mixed DOC/table join returned no owner")
	}
	t.Cleanup(func() { astTestMust(t, product.Close()) })
	return product
}

func assertUDocTableProductContains(t *testing.T, product *UDocTableProduct, want bool, values ...ast.ASTHandle) {
	t.Helper()
	found, err := product.ContainsFact(values)
	astTestMust(t, err)
	if found != want {
		t.Fatal("mixed DOC/table product membership", found, want)
	}
}

func assertUDocTableProductFormula(t *testing.T, manager *ast.ASTManager, product *UDocTableProduct, rows ...[]ast.ASTHandle) {
	t.Helper()
	ref, err := product.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.DelRef(&ref)) }()
	actual, err := manager.RefGet(ref)
	astTestMust(t, err)
	sorts := product.Sorts()
	variables := make([]ast.ASTHandle, len(sorts))
	for i, sort := range sorts {
		variables[i], err = manager.MkVar(uint32(i), sort)
		astTestMust(t, err)
		astTestPin(t, manager, variables[i])
	}
	disjuncts := make([]ast.ASTHandle, len(rows))
	for i, row := range rows {
		if len(row) != len(sorts) {
			t.Fatal("test oracle row arity", len(row), len(sorts))
		}
		terms := make([]ast.ASTHandle, len(row))
		for column, value := range row {
			terms[column], err = manager.MkEq(variables[column], value)
			astTestMust(t, err)
			astTestPin(t, manager, terms[column])
		}
		disjuncts[i], err = manager.MkAndArray(terms)
		astTestMust(t, err)
		astTestPin(t, manager, disjuncts[i])
	}
	expected, err := manager.MkOrArray(disjuncts)
	astTestMust(t, err)
	astTestPin(t, manager, expected)
	oracle := &relationFormulaChecker{manager: manager, sorts: sorts, config: formulaCheckerConfig(t, manager)}
	astTestMust(t, oracle.verifyGrounded(expected, actual, false))
}

func udocTableProductFixture(t *testing.T) (*tableRelationFixture, *UDocRelationFactory, *TypedUDocRelation, []ast.ASTHandle) {
	t.Helper()
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	raw, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
	doc := ownUDocTableUnionRelation(t, raw, err)
	values := make([]ast.ASTHandle, 4)
	for i := range values {
		values[i] = f.bv(uint64(i))
		astTestPin(t, f.ast, values[i])
	}
	return f, factory, doc, values
}

func udocTableProductFinite(t *testing.T, domains []uint64, facts ...Fact) FiniteTable {
	t.Helper()
	signature, err := NewSignature(domains)
	astTestMust(t, err)
	finite := NewHashTable(signature)
	t.Cleanup(func() { astTestMust(t, finite.Close()) })
	for _, fact := range facts {
		_, err := finite.Add(fact)
		astTestMust(t, err)
	}
	return finite
}

func TestUDocTableProductJoinsBothOrientationsAndPreservesConcreteComponents(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, keyed := range []bool{false, true} {
			name := "doc-first"
			if reverse {
				name = "table-first"
			}
			if keyed {
				name += "/selected-key"
			} else {
				name += "/cartesian"
			}
			t.Run(name, func(t *testing.T) {
				f, _, doc, values := udocTableProductFixture(t)
				checkedUDocAdd(t, doc, values[1])
				finite := udocTableProductFinite(t, []uint64{2, 4}, Fact{1, 1}, Fact{0, 2})
				finiteSorts := []ast.ASTHandle{f.boolean, f.bits}
				var docColumns, tableColumns []uint32
				if keyed {
					docColumns, tableColumns = []uint32{0}, []uint32{1}
				}
				var raw *UDocTableProduct
				var err error
				if reverse {
					raw, err = JoinFiniteUDoc(f.relations, finiteSorts, finite, doc, tableColumns, docColumns)
				} else {
					raw, err = JoinUDocFinite(f.relations, doc, finiteSorts, finite, docColumns, tableColumns)
				}
				product := ownUDocTableProduct(t, raw, err)
				wantSorts := []ast.ASTHandle{f.bits, f.boolean, f.bits}
				wantMask := []bool{true, false, true}
				if reverse {
					wantSorts = []ast.ASTHandle{f.boolean, f.bits, f.bits}
					wantMask = []bool{false, true, true}
				}
				if len(product.Sorts()) != len(wantSorts) {
					t.Fatal("mixed join changed source arity", product.Sorts(), wantSorts)
				}
				for i, sort := range product.Sorts() {
					if sort != wantSorts[i] {
						t.Fatal("mixed join changed source column order", product.Sorts(), wantSorts)
					}
				}
				assertUDocSieveMask(t, product.Doc(), wantMask...)
				if product.Doc().Inner() == doc || product.Table() == finite {
					t.Fatal("mixed join aliased an input owner")
				}
				var rows [][]ast.ASTHandle
				for d, docValue := range values {
					for b, boolean := range []ast.ASTHandle{f.falsehood, f.truth} {
						for k, key := range values {
							want := d == 1 && ((b == 1 && k == 1) || (b == 0 && k == 2)) && (!keyed || d == k)
							row := []ast.ASTHandle{docValue, boolean, key}
							if reverse {
								row = []ast.ASTHandle{boolean, key, docValue}
							}
							assertUDocTableProductContains(t, product, want, row...)
							if want {
								rows = append(rows, row)
							}
						}
					}
					checkedUDocContains(t, doc, d == 1, docValue)
				}
				assertUDocTableProductFormula(t, f.ast, product, rows...)
				if count, err := finite.Len(); err != nil || count != 2 {
					t.Fatal("mixed join changed its finite input", count, err)
				}
			})
		}
	}
}

func TestUDocTableProductSourceProjectionLosesCrossComponentCorrelation(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	checkedUDocAdd(t, doc, values[1])
	finite := udocTableProductFinite(t, []uint64{2, 4}, Fact{1, 2})
	raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean, f.bits}, finite, []uint32{0}, []uint32{1})
	product := ownUDocTableProduct(t, raw, err)
	// Source emptiness tests the components separately. Their conjunction is
	// false, but neither component is empty before the common key is removed.
	if empty, err := product.Empty(); err != nil || empty {
		t.Fatal("product replaced source component emptiness with exact intersection", empty, err)
	}
	assertUDocTableProductFormula(t, f.ast, product)
	for _, value := range values {
		assertUDocTableProductContains(t, product, false, value, f.truth, value)
	}
	raw, err = product.Project([]uint32{0, 2})
	projected := ownUDocTableProduct(t, raw, err)
	assertUDocSieveMask(t, projected.Doc(), false)
	assertUDocTableProductContains(t, projected, true, f.truth)
	assertUDocTableProductContains(t, projected, false, f.falsehood)
	assertUDocTableProductFormula(t, f.ast, projected, []ast.ASTHandle{f.truth})
	raw, err = product.Project([]uint32{0, 1, 2})
	nullary := ownUDocTableProduct(t, raw, err)
	assertUDocTableProductContains(t, nullary, true)
	assertUDocTableProductFormula(t, f.ast, nullary, nil)
	assertUDocTableProductFormula(t, f.ast, product)
}

func TestUDocTableProductEmptyProjectionKeepsFalseInnerComponent(t *testing.T) {
	f, _, doc, _ := udocTableProductFixture(t)
	finite := udocTableProductFinite(t, []uint64{2}, Fact{1})
	raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean}, finite, nil, nil)
	product := ownUDocTableProduct(t, raw, err)
	for _, removed := range [][]uint32{nil, {0}, {0, 1}} {
		raw, err := product.Project(removed)
		projected := ownUDocTableProduct(t, raw, err)
		if empty, err := projected.Empty(); err != nil || !empty {
			t.Fatal("projection replaced empty DOC with full ignored columns", removed, empty, err)
		}
		assertUDocTableProductFormula(t, f.ast, projected)
	}
}

func TestUDocTableProductCloneProjectionAndFormulaOwnTheirLifetimes(t *testing.T) {
	f, _, doc, values := udocTableProductFixture(t)
	checkedUDocAdd(t, doc, values[3])
	finite := udocTableProductFinite(t, []uint64{2}, Fact{0})
	raw, err := JoinFiniteUDoc(f.relations, []ast.ASTHandle{f.boolean}, finite, doc, nil, nil)
	product := ownUDocTableProduct(t, raw, err)
	raw, err = product.Clone()
	clone := ownUDocTableProduct(t, raw, err)
	raw, err = product.Project([]uint32{0})
	projected := ownUDocTableProduct(t, raw, err)
	answer, err := clone.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&answer)) }()
	astTestMust(t, product.Close())
	astTestMust(t, doc.Close())
	astTestMust(t, finite.Close())
	assertUDocTableProductContains(t, clone, true, f.falsehood, values[3])
	assertUDocTableProductContains(t, projected, true, values[3])
	assertUDocTableProductContains(t, projected, false, values[2])
	assertUDocTableProductFormula(t, f.ast, projected, []ast.ASTHandle{values[3]})
	astTestMust(t, clone.Doc().Inner().Reset())
	assertUDocTableProductContains(t, projected, true, values[3])
	astTestMust(t, clone.Close())
	astTestMust(t, projected.Close())
	if _, err := f.ast.RefGet(answer); err != nil {
		t.Fatal("owning product formula died with components", err)
	}
}

func TestUDocTableProductRejectsUnsupportedSourceShapesWithoutMutation(t *testing.T) {
	f, factory, doc, values := udocTableProductFixture(t)
	checkedUDocAdd(t, doc, values[1])
	finite := udocTableProductFinite(t, []uint64{2}, Fact{1})
	checkedFactory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	t.Cleanup(checkedFactory.Close)
	raw, err := checkedFactory.NewEmpty([]ast.ASTHandle{f.bits})
	checked := ownUDocTableUnionRelation(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{f.bits, f.bits})
	multiple := ownUDocTableUnionRelation(t, raw, err)
	wideSort, err := f.ast.BitVectorSort(96)
	astTestMust(t, err)
	astTestPin(t, f.ast, wideSort)
	raw, err = factory.NewEmpty([]ast.ASTHandle{wideSort})
	wide := ownUDocTableUnionRelation(t, raw, err)
	for _, test := range []struct {
		name  string
		doc   *TypedUDocRelation
		sorts []ast.ASTHandle
		table FiniteTable
		left  []uint32
		right []uint32
	}{
		{"checked", checked, []ast.ASTHandle{f.boolean}, finite, nil, nil},
		{"multiple-doc-columns", multiple, []ast.ASTHandle{f.boolean}, finite, nil, nil},
		{"wide-unmaterializable-opposite", wide, []ast.ASTHandle{f.boolean}, finite, nil, nil},
		{"finite-signature-mismatch", doc, []ast.ASTHandle{f.boolean, f.bits}, finite, nil, nil},
		{"incompatible-key-sorts", doc, []ast.ASTHandle{f.boolean}, finite, []uint32{0}, []uint32{0}},
		{"outside-key-column", doc, []ast.ASTHandle{f.boolean}, finite, []uint32{1}, []uint32{0}},
		{"mismatched-key-count", doc, []ast.ASTHandle{f.boolean}, finite, []uint32{0}, nil},
		{"all-doc-finite-side", doc, []ast.ASTHandle{f.bits}, udocTableProductFinite(t, []uint64{4}, Fact{1}), nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			product, err := JoinUDocFinite(f.relations, test.doc, test.sorts, test.table, test.left, test.right)
			if err == nil || product != nil {
				if product != nil {
					_ = product.Close()
				}
				t.Fatal("unsupported mixed join published an owner", err)
			}
			checkedUDocContains(t, doc, true, values[1])
			if count, err := finite.Len(); err != nil || count != 1 {
				t.Fatal("mixed join rejection changed borrowed finite source", count, err)
			}
		})
	}
}

func TestUDocTableProductPreservesSparseAndLazyFiniteKinds(t *testing.T) {
	for _, kind := range []string{"sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f, _, doc, values := udocTableProductFixture(t)
			checkedUDocAdd(t, doc, values[2])
			if kind == "lazy_sparse" {
				factory := NewLazySparseFiniteFactory()
				t.Cleanup(func() { astTestMust(t, factory.Close()) })
				astTestMust(t, f.relations.SetLazySparseFiniteFactory(factory))
			}
			astTestMust(t, f.relations.SetDefaultFiniteTable(kind))
			signature, err := NewSignature([]uint64{2, 4})
			astTestMust(t, err)
			finite, err := f.relations.NewDefaultFiniteTable(signature)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, finite.Close()) })
			_, err = finite.Add(Fact{1, 2})
			astTestMust(t, err)
			raw, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean, f.bits}, finite, []uint32{0}, []uint32{1})
			product := ownUDocTableProduct(t, raw, err)
			switch kind {
			case "sparse":
				if _, ok := product.Table().(*SparseTable); !ok {
					t.Fatalf("mixed join changed sparse storage to %T", product.Table())
				}
			case "lazy_sparse":
				if _, ok := product.Table().(*LazySparseTable); !ok {
					t.Fatalf("mixed join changed lazy storage to %T", product.Table())
				}
			}
			assertUDocTableProductContains(t, product, true, values[2], f.truth, values[2])
			assertUDocTableProductContains(t, product, false, values[2], f.falsehood, values[2])
			assertUDocTableProductFormula(t, f.ast, product, []ast.ASTHandle{values[2], f.truth, values[2]})
			raw, err = product.Project([]uint32{0, 1, 2})
			nullary := ownUDocTableProduct(t, raw, err)
			assertUDocTableProductContains(t, nullary, true)
			assertUDocTableProductFormula(t, f.ast, nullary, nil)
			astTestMust(t, product.Close())
			if found, err := finite.Contains(Fact{1, 2}); err != nil || !found {
				t.Fatal("closing mixed product consumed its borrowed finite input", found, err)
			}
		})
	}
}
