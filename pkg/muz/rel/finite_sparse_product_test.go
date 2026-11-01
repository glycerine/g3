package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductFactsAndClone(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if empty, e := product.Empty(); e != nil || !empty {
		t.Fatalf("new product empty: got %v (%v)", empty, e)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, one}} {
		if err = product.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{{[]ast.ASTHandle{zero, one}, true}, {[]ast.ASTHandle{zero, three}, false}} {
		found, e := product.ContainsFact(tc.fact)
		if e != nil || found != tc.want {
			t.Fatalf("contains %v: got %v want %v (%v)", tc.fact, found, tc.want, e)
		}
	}
	if empty, e := product.Empty(); e != nil || empty {
		t.Fatalf("populated product empty: got %v (%v)", empty, e)
	}
	clone, err := product.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	if err = clone.AddFact([]ast.ASTHandle{zero, three}); err != nil {
		t.Fatal(err)
	}
	if found, err := product.ContainsFact([]ast.ASTHandle{zero, three}); err != nil || found {
		t.Fatal("clone mutation escaped into source", found, err)
	}
	if found, err := clone.ContainsFact([]ast.ASTHandle{zero, three}); err != nil || !found {
		t.Fatal("clone did not accept inner fact", found, err)
	}
}

func TestFiniteSparseProductRejectsInvalidConstructionAndFacts(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-invalid-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 2)
	astTestMust(t, err)
	if _, err = NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true}); err == nil {
		t.Fatal("mismatched column mask accepted")
	}
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if err = product.AddFact(nil); err == nil {
		t.Fatal("wrong-arity fact accepted")
	}
}

func TestFiniteSparseProductUnionFrom(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-union-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	target, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	source, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{zero, two}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{one, one}))
	if err = target.UnionFrom(source); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, one}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("union missing %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := target.ContainsFact([]ast.ASTHandle{one, two}); e != nil || found {
		t.Fatalf("union added absent row: %v (%v)", found, e)
	}
	if found, e := source.ContainsFact([]ast.ASTHandle{zero, one}); e != nil || found {
		t.Fatalf("union mutated source: %v (%v)", found, e)
	}
}

func TestFiniteSparseProductUnionFromWithDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-delta-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	newProduct := func() *FiniteSparseProduct {
		product, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
		astTestMust(t, e)
		return product
	}
	target, source, delta := newProduct(), newProduct(), newProduct()
	defer func() { astTestMust(t, target.Close()) }()
	defer func() { astTestMust(t, source.Close()) }()
	defer func() { astTestMust(t, delta.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{zero, one}))
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, three}} {
		astTestMust(t, source.AddFact(fact))
	}
	astTestMust(t, delta.AddFact([]ast.ASTHandle{three, zero}))
	if err = target.UnionFromWithDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, three}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("union target missing %v: %v (%v)", fact, found, e)
		}
	}
	for _, fact := range [][]ast.ASTHandle{{zero, two}, {one, three}, {three, zero}} {
		if found, e := delta.ContainsFact(fact); e != nil || !found {
			t.Fatalf("union delta missing %v: %v (%v)", fact, found, e)
		}
	}
	flat, err := delta.flatten()
	astTestMust(t, err)
	if count, e := flat.Len(); e != nil || count != 3 {
		t.Fatalf("delta contains %d facts, want 3 (%v)", count, e)
	}
	astTestMust(t, flat.Close())
	if err = target.UnionFromWithDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	flat, err = delta.flatten()
	astTestMust(t, err)
	if count, e := flat.Len(); e != nil || count != 3 {
		t.Fatalf("repeated union changed delta cardinality to %d (%v)", count, e)
	}
	astTestMust(t, flat.Close())
}

func TestFiniteSparseProductProjectMergesRows(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-project-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	source, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort, sort}, []bool{true, false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one, two}, {three, one, two}, {zero, two, three}} {
		astTestMust(t, source.AddFact(fact))
	}
	projected, err := source.Project([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{one, two}, {two, three}} {
		if found, e := projected.ContainsFact(fact); e != nil || !found {
			t.Fatalf("projected product missing %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := projected.ContainsFact([]ast.ASTHandle{three, one}); e != nil || found {
		t.Fatalf("projection added absent row: %v (%v)", found, e)
	}
	for _, removed := range [][]int{{1, 0}, {3}} {
		if _, e := source.Project(removed); e == nil {
			t.Fatalf("invalid projection columns accepted: %v", removed)
		}
	}
}

func TestFiniteSparseProductJoinAcrossPartitions(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-join-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	left, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, left.Close()) }()
	right, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, right.Close()) }()
	astTestMust(t, left.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, left.AddFact([]ast.ASTHandle{one, two}))
	astTestMust(t, right.AddFact([]ast.ASTHandle{one, three}))
	astTestMust(t, right.AddFact([]ast.ASTHandle{two, zero}))
	joined, err := left.Join(right, []int{1}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one, one, three}, {one, two, two, zero}} {
		if found, e := joined.ContainsFact(fact); e != nil || !found {
			t.Fatalf("join missing %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := joined.ContainsFact([]ast.ASTHandle{zero, one, two, zero}); e != nil || found {
		t.Fatalf("join produced a mismatched pair: %v (%v)", found, e)
	}
	for _, leftTable := range []bool{false, true} {
		for _, rightTable := range []bool{false, true} {
			leftPart, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, []bool{leftTable})
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, leftPart.Close()) })
			rightPart, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, []bool{rightTable})
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, rightPart.Close()) })
			astTestMust(t, leftPart.AddFact([]ast.ASTHandle{zero}))
			astTestMust(t, leftPart.AddFact([]ast.ASTHandle{one}))
			astTestMust(t, rightPart.AddFact([]ast.ASTHandle{one}))
			partJoin, e := leftPart.Join(rightPart, []int{0}, []int{0})
			if e != nil {
				t.Fatalf("join table partitions (%v,%v): %v", leftTable, rightTable, e)
			}
			t.Cleanup(func() { astTestMust(t, partJoin.Close()) })
			if found, e := partJoin.ContainsFact([]ast.ASTHandle{one, one}); e != nil || !found {
				t.Fatalf("join table partitions (%v,%v) lost match: %v (%v)", leftTable, rightTable, found, e)
			}
			if found, e := partJoin.ContainsFact([]ast.ASTHandle{zero, one}); e != nil || found {
				t.Fatalf("join table partitions (%v,%v) accepted mismatch: %v (%v)", leftTable, rightTable, found, e)
			}
		}
	}
}

func TestFiniteSparseProductJoinProjectComposition(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-join-project")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	left, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, left.Close()) }()
	right, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, right.Close()) }()
	astTestMust(t, left.AddFact([]ast.ASTHandle{zero, zero}))
	astTestMust(t, left.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, right.AddFact([]ast.ASTHandle{zero, two}))
	astTestMust(t, right.AddFact([]ast.ASTHandle{one, two}))

	projected, err := left.JoinProject(right, []int{1}, []int{0}, []int{1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if rows, e := projected.flatten(); e != nil {
		t.Fatal(e)
	} else {
		if count, e := rows.Len(); e != nil || count != 1 {
			t.Fatalf("join-project rows=%d, want 1 (%v)", count, e)
		}
		astTestMust(t, rows.Close())
	}
	if found, e := projected.ContainsFact([]ast.ASTHandle{zero, two}); e != nil || !found {
		t.Fatalf("join-project lost projected fact: %v (%v)", found, e)
	}
	if found, e := left.ContainsFact([]ast.ASTHandle{zero, one}); e != nil || !found {
		t.Fatalf("join-project mutated left input: %v (%v)", found, e)
	}
	if _, err = left.JoinProject(right, []int{1}, []int{0}, []int{2, 1}); err == nil {
		t.Fatal("unsorted projection columns accepted")
	}
}

func TestFiniteSparseProductJoinPartitionOracle(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-join-partition-oracle")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	values := make([]ast.ASTHandle, 3)
	for i := range values {
		values[i], err = f.ast.MkDatalogNumeral(uint64(i), sort)
		astTestMust(t, err)
		astTestPin(t, f.ast, values[i])
	}
	conditions := []struct{ left, right []int }{
		{nil, nil},
		{[]int{0}, []int{0}},
		{[]int{0, 1}, []int{1, 0}},
	}
	for leftLayout := 0; leftLayout < 4; leftLayout++ {
		for rightLayout := 0; rightLayout < 4; rightLayout++ {
			leftMask := []bool{leftLayout&1 != 0, leftLayout&2 != 0}
			rightMask := []bool{rightLayout&1 != 0, rightLayout&2 != 0}
			left, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, leftMask)
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, left.Close()) })
			right, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, rightMask)
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, right.Close()) })
			var leftRows, rightRows []Fact
			for a := Element(0); a < 3; a++ {
				for b := Element(0); b < 3; b++ {
					row := Fact{a, b}
					if (int(a)*5+int(b)*3+leftLayout)%4 != 0 {
						astTestMust(t, left.AddFact([]ast.ASTHandle{values[a], values[b]}))
						leftRows = append(leftRows, row)
					}
					if (int(a)*2+int(b)+rightLayout)%3 != 0 {
						astTestMust(t, right.AddFact([]ast.ASTHandle{values[a], values[b]}))
						rightRows = append(rightRows, row)
					}
				}
			}
			for _, condition := range conditions {
				joined, e := left.Join(right, condition.left, condition.right)
				if e != nil {
					t.Fatalf("layouts (%d,%d), join %v/%v: %v", leftLayout, rightLayout, condition.left, condition.right, e)
				}
				t.Cleanup(func() { astTestMust(t, joined.Close()) })
				want := make(map[string]struct{})
				for _, x := range leftRows {
					for _, y := range rightRows {
						matches := true
						for i := range condition.left {
							if x[condition.left[i]] != y[condition.right[i]] {
								matches = false
								break
							}
						}
						if matches {
							want[factKey(append(append(Fact(nil), x...), y...))] = struct{}{}
						}
					}
				}
				for a := Element(0); a < 3; a++ {
					for b := Element(0); b < 3; b++ {
						for c := Element(0); c < 3; c++ {
							for d := Element(0); d < 3; d++ {
								row := Fact{a, b, c, d}
								found, containsErr := joined.ContainsFact([]ast.ASTHandle{values[a], values[b], values[c], values[d]})
								_, expected := want[factKey(row)]
								if containsErr != nil || found != expected {
									t.Fatalf("layouts (%d,%d), join %v/%v, row %v: got %v err %v, want %v", leftLayout, rightLayout, condition.left, condition.right, row, found, containsErr, expected)
								}
							}
						}
					}
				}
				astTestMust(t, joined.Close())
			}
			astTestMust(t, left.Close())
			astTestMust(t, right.Close())
		}
	}
}

func TestFiniteSparseProductEqualityFilters(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-filter-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort, sort}, []bool{true, false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, zero, one}, {one, two, three}, {two, two, three}} {
		astTestMust(t, product.AddFact(fact))
	}
	if err = product.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err = product.FilterEqual(2, three); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]ast.ASTHandle{{two, two, three}} {
		if found, e := product.ContainsFact(fact); e != nil || !found {
			t.Fatalf("equality filters removed matching row %v: %v (%v)", fact, found, e)
		}
	}
	for _, fact := range [][]ast.ASTHandle{{zero, zero, one}, {one, two, three}} {
		if found, e := product.ContainsFact(fact); e != nil || found {
			t.Fatalf("equality filters kept rejected row %v: %v (%v)", fact, found, e)
		}
	}
	otherName, err := f.ast.Symbols.Intern("finite-sparse-product-filter-other-domain")
	astTestMust(t, err)
	otherSort, err := f.ast.MkDatalogFiniteSort(otherName, 4)
	astTestMust(t, err)
	wrongSortValue, err := f.ast.MkDatalogNumeral(3, otherSort)
	astTestMust(t, err)
	astTestPin(t, f.ast, wrongSortValue)
	if err = product.FilterEqual(0, wrongSortValue); err == nil {
		t.Fatal("sort-mismatched value accepted")
	}
	crossSort, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, otherSort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, crossSort.Close()) }()
	astTestMust(t, crossSort.AddFact([]ast.ASTHandle{one, wrongSortValue}))
	if err = crossSort.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if empty, e := crossSort.Empty(); e != nil || !empty {
		t.Fatalf("different-sort columns treated as identical: empty=%v err=%v", empty, e)
	}
}

func TestFiniteSparseProductNegationFilterAcrossPartitions(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-negation-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	target, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	negative, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, negative.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {one, two}, {one, three}} {
		astTestMust(t, target.AddFact(fact))
	}
	astTestMust(t, negative.AddFact([]ast.ASTHandle{one, three}))
	removed, err := target.FilterNegation(negative, []int{0, 1}, []int{0, 1})
	if err != nil || removed != 1 {
		t.Fatalf("negation removed %d rows, want 1 (%v)", removed, err)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {one, two}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("negation removed unrelated fact %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := target.ContainsFact([]ast.ASTHandle{one, three}); e != nil || found {
		t.Fatalf("negation kept excluded fact: %v (%v)", found, e)
	}
	if _, err = target.FilterNegation(negative, []int{0}, []int{2}); err == nil {
		t.Fatal("out-of-range negation column accepted")
	}
}

func TestFiniteSparseProductRenameMovesColumnsAndPartitions(t *testing.T) {
	f := newTableRelationFixture(t)
	firstName, err := f.ast.Symbols.Intern("finite-sparse-product-rename-first")
	astTestMust(t, err)
	firstSort, err := f.ast.MkDatalogFiniteSort(firstName, 3)
	astTestMust(t, err)
	secondName, err := f.ast.Symbols.Intern("finite-sparse-product-rename-second")
	astTestMust(t, err)
	secondSort, err := f.ast.MkDatalogFiniteSort(secondName, 5)
	astTestMust(t, err)
	value := func(sort ast.ASTHandle, n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	a1, a2 := value(firstSort, 1), value(firstSort, 2)
	b2, b4 := value(secondSort, 2), value(secondSort, 4)
	source, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{firstSort, secondSort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, source.AddFact([]ast.ASTHandle{a1, b2}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{a2, b4}))
	renamed, err := source.Rename([]int{0, 1})
	astTestMust(t, err)
	defer func() { astTestMust(t, renamed.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{b2, a1}, {b4, a2}} {
		if found, e := renamed.ContainsFact(fact); e != nil || !found {
			t.Fatalf("renamed product missing %v: %v (%v)", fact, found, e)
		}
	}
	if len(renamed.tableColumns) != 1 || renamed.tableColumns[0] != 1 || len(renamed.innerCols) != 1 || renamed.innerCols[0] != 0 {
		t.Fatalf("renamed partition = table %v inner %v", renamed.tableColumns, renamed.innerCols)
	}
	for _, cycle := range [][]int{{0}, {0, 0}, {0, 2}} {
		if _, e := source.Rename(cycle); e == nil {
			t.Fatalf("invalid rename cycle accepted: %v", cycle)
		}
	}
}

func TestFiniteSparseProductComplementAndFull(t *testing.T) {
	f := newTableRelationFixture(t)
	firstName, err := f.ast.Symbols.Intern("finite-sparse-product-complement-first")
	astTestMust(t, err)
	firstSort, err := f.ast.MkDatalogFiniteSort(firstName, 2)
	astTestMust(t, err)
	secondName, err := f.ast.Symbols.Intern("finite-sparse-product-complement-second")
	astTestMust(t, err)
	secondSort, err := f.ast.MkDatalogFiniteSort(secondName, 3)
	astTestMust(t, err)
	value := func(sort ast.ASTHandle, n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zeroA, oneA := value(firstSort, 0), value(firstSort, 1)
	zeroB, oneB, twoB := value(secondSort, 0), value(secondSort, 1), value(secondSort, 2)
	sorts := []ast.ASTHandle{firstSort, secondSort}
	product, err := NewFiniteSparseProduct(f.relations, sorts, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{zeroA, zeroB}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{oneA, twoB}))
	complement, err := product.Complement()
	astTestMust(t, err)
	defer func() { astTestMust(t, complement.Close()) }()
	if rows, e := complement.flatten(); e != nil {
		t.Fatal(e)
	} else {
		if count, e := rows.Len(); e != nil || count != 4 {
			t.Fatalf("complement rows = %d, want 4 (%v)", count, e)
		}
		astTestMust(t, rows.Close())
	}
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{{[]ast.ASTHandle{zeroA, zeroB}, false}, {[]ast.ASTHandle{oneA, twoB}, false}, {[]ast.ASTHandle{zeroA, oneB}, true}, {[]ast.ASTHandle{oneA, zeroB}, true}} {
		if found, e := complement.ContainsFact(tc.fact); e != nil || found != tc.want {
			t.Fatalf("complement contains %v = %v, want %v (%v)", tc.fact, found, tc.want, e)
		}
	}
	if err = product.ComplementSelf(); err != nil {
		t.Fatal(err)
	}
	if found, e := product.ContainsFact([]ast.ASTHandle{zeroA, oneB}); e != nil || !found {
		t.Fatalf("complemented source should contain absent fact: %v (%v)", found, e)
	}
	if found, e := product.ContainsFact([]ast.ASTHandle{zeroA, zeroB}); e != nil || found {
		t.Fatalf("complemented source should exclude original fact: %v (%v)", found, e)
	}
	full, err := NewFullFiniteSparseProduct(f.relations, sorts, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, full.Close()) }()
	if rows, e := full.flatten(); e != nil {
		t.Fatal(e)
	} else {
		if count, e := rows.Len(); e != nil || count != 6 {
			t.Fatalf("full rows = %d, want 6 (%v)", count, e)
		}
		astTestMust(t, rows.Close())
	}
	if _, err = NewFullFiniteSparseProduct(f.relations, []ast.ASTHandle{firstSort, firstSort, secondSort}, []bool{true, true, false}); err == nil {
		t.Fatal("complement unsupported multi-column partition")
	}
	zeroArity, err := NewFiniteSparseProduct(f.relations, nil, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, zeroArity.Close()) }()
	zeroArityFull, err := zeroArity.Complement()
	astTestMust(t, err)
	defer func() { astTestMust(t, zeroArityFull.Close()) }()
	if found, e := zeroArityFull.ContainsFact(nil); e != nil || !found {
		t.Fatalf("zero-arity complement missing empty row: %v (%v)", found, e)
	}
}

func TestFiniteSparseProductIdenticalPairs(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-identical-pairs")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort, sort, sort}, []bool{true, false, true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, zero, one, one}, {zero, one, one, one}, {two, two, two, zero}} {
		astTestMust(t, product.AddFact(fact))
	}
	if err = product.FilterIdenticalPairs([][2]int{{0, 1}, {2, 3}}); err != nil {
		t.Fatal(err)
	}
	if found, e := product.ContainsFact([]ast.ASTHandle{zero, zero, one, one}); e != nil || !found {
		t.Fatalf("pair filter removed matching row: %v (%v)", found, e)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one, one, one}, {two, two, two, zero}} {
		if found, e := product.ContainsFact(fact); e != nil || found {
			t.Fatalf("pair filter kept mismatch %v: %v (%v)", fact, found, e)
		}
	}
	if err = product.FilterIdenticalPairs([][2]int{{0, 9}}); err == nil {
		t.Fatal("invalid identity pair accepted")
	}
}

func TestFiniteSparseProductUnionUnifiesTableSpecification(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-unify-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one := value(0), value(1)
	newProduct := func(mask []bool) *FiniteSparseProduct {
		p, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, mask)
		astTestMust(t, e)
		return p
	}
	target := newProduct([]bool{true, false})
	source := newProduct([]bool{false, true})
	delta := newProduct([]bool{true, true})
	defer func() { astTestMust(t, target.Close()) }()
	defer func() { astTestMust(t, source.Close()) }()
	defer func() { astTestMust(t, delta.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{zero, zero}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{zero, zero}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, delta.AddFact([]ast.ASTHandle{one, zero}))
	if err = target.UnionFromWithDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	if len(target.tableColumns) != 0 || len(source.tableColumns) != 1 || source.tableColumns[0] != 1 || len(delta.tableColumns) != 0 {
		t.Fatalf("union partition target=%v source=%v delta=%v", target.tableColumns, source.tableColumns, delta.tableColumns)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, zero}, {zero, one}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("unified target missing %v: %v (%v)", fact, found, e)
		}
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {one, zero}} {
		if found, e := delta.ContainsFact(fact); e != nil || !found {
			t.Fatalf("unified delta missing %v: %v (%v)", fact, found, e)
		}
	}
	plainTarget, plainSource := newProduct([]bool{true, false}), newProduct([]bool{false, true})
	defer func() { astTestMust(t, plainTarget.Close()) }()
	defer func() { astTestMust(t, plainSource.Close()) }()
	astTestMust(t, plainTarget.AddFact([]ast.ASTHandle{zero, zero}))
	astTestMust(t, plainSource.AddFact([]ast.ASTHandle{one, one}))
	if err = plainTarget.UnionFrom(plainSource); err != nil {
		t.Fatal(err)
	}
	if len(plainTarget.tableColumns) != 0 {
		t.Fatalf("plain union did not use common partition: %v", plainTarget.tableColumns)
	}
	if found, e := plainTarget.ContainsFact([]ast.ASTHandle{one, one}); e != nil || !found {
		t.Fatalf("plain unified union missing new row: %v (%v)", found, e)
	}
}

func TestFiniteSparseProductSizeEstimatesUseOuterTable(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-estimate-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if rows, e := product.SizeEstimateRows(); e != nil || rows != 0 {
		t.Fatalf("empty outer row estimate = %d (%v)", rows, e)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, one}} {
		astTestMust(t, product.AddFact(fact))
	}
	if rows, e := product.SizeEstimateRows(); e != nil || rows != 2 {
		t.Fatalf("outer row estimate = %d, want 2 (%v)", rows, e)
	}
	wantBytes := uint32(len(product.table.data.entries)) * product.table.layout.EntryBytes()
	if bytes, e := product.SizeEstimateBytes(); e != nil || bytes != wantBytes {
		t.Fatalf("outer byte estimate = %d, want %d (%v)", bytes, wantBytes, e)
	}
}

func TestFiniteSparseProductEmptyCollectsSharedInnerIndexes(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-gc-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one := value(0), value(1)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{one, one}))
	keys, err := product.table.Facts()
	astTestMust(t, err)
	keys[1][len(keys[1])-1] = keys[0][len(keys[0])-1]
	astTestMust(t, product.table.Reset())
	for _, key := range keys {
		astTestMust(t, product.table.EnsureFact(key))
	}
	astTestMust(t, product.inner[0].Reset())
	if empty, err := product.Empty(); err != nil || !empty {
		t.Fatalf("empty collection result = %v (%v)", empty, err)
	}
	if rows, err := product.table.Len(); err != nil || rows != 0 {
		t.Fatalf("empty collection left %d outer keys (%v)", rows, err)
	}
	if len(product.inner) != 0 || len(product.available) != 0 {
		t.Fatalf("garbage collection kept inner indexes: inner=%d available=%d", len(product.inner), len(product.available))
	}
}
