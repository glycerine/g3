package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveTargetTableNegationUsesOuterMappingsAndFullIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits, f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	newTarget := func() *SieveRelation {
		t.Helper()
		target, newErr := NewSieveRelation(signature, []bool{true, false, true})
		if newErr != nil {
			t.Fatal(newErr)
		}
		for _, fact := range []Fact{{0, 0, 0}, {0, 0, 1}, {1, 0, 0}, {1, 0, 1}} {
			if _, newErr = target.AddFact(fact); newErr != nil {
				t.Fatal(newErr)
			}
		}
		return target
	}
	negative, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	defer negative.Close()
	if _, err = negative.AddTableFact(Fact{1}); err != nil {
		t.Fatal(err)
	}
	target := newTarget()
	defer target.Close()
	// Outer target column 2 maps to inner column 1, not equality-pair index 0.
	if err = target.FilterByTableNegation(negative, []int{2}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if target.NumInnerRows() != 2 {
		t.Fatal("outer-column negation rows", target.NumInnerRows())
	}
	for ignored := Element(0); ignored < 4; ignored++ {
		requireSieveFact(t, target, Fact{0, ignored, 0}, true)
		requireSieveFact(t, target, Fact{1, ignored, 0}, true)
		requireSieveFact(t, target, Fact{0, ignored, 1}, false)
	}
	if negative.NumRows() != 1 {
		t.Fatal("negative source mutated", negative.NumRows())
	}

	identity := newTarget()
	defer identity.Close()
	if err = identity.FilterByTableNegation(negative, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if identity.NumInnerRows() != 4 {
		t.Fatal("full target endpoint must make filter identity", identity.NumInnerRows())
	}
	if err = negative.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = identity.ContainsFact(Fact{1, 3, 1}); err != nil {
		t.Fatal("target borrowed negative owner", err)
	}
	if err = identity.FilterByTableNegation(negative, nil, nil); !errors.Is(err, ErrSieve) {
		t.Fatal("closed negative accepted", err)
	}
}

func TestTableTargetSieveNegationSelectedAndFullEndpoints(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits, f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	newTarget := func() *TableRelation {
		t.Helper()
		target, newErr := NewTableRelation(f.relations, sorts)
		if newErr != nil {
			t.Fatal(newErr)
		}
		for _, fact := range []Fact{{0, 0, 0}, {0, 1, 1}, {1, 2, 0}, {1, 3, 1}} {
			if _, newErr = target.AddTableFact(fact); newErr != nil {
				t.Fatal(newErr)
			}
		}
		return target
	}
	negative, err := NewSieveRelation(signature, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	defer negative.Close()
	if _, err = negative.AddFact(Fact{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	target := newTarget()
	defer target.Close()
	// The full negative middle column is skipped; selected outer column 2 maps
	// to inner column 1 and removes both target rows whose third value is 0.
	if err = negative.FilterTableByNegation(target, []int{1, 2}, []int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if target.NumRows() != 2 {
		t.Fatal("selected/full mixed negation rows", target.NumRows())
	}
	for _, fact := range []Fact{{0, 1, 1}, {1, 3, 1}} {
		found, findErr := target.Table().Contains(fact)
		if findErr != nil || !found {
			t.Fatal("expected target row", fact, found, findErr)
		}
	}
	if negative.NumInnerRows() != 1 {
		t.Fatal("negative sieve mutated", negative.NumInnerRows())
	}

	fullNegative, err := NewFullSieve(signature)
	if err != nil {
		t.Fatal(err)
	}
	defer fullNegative.Close()
	allRemoved := newTarget()
	defer allRemoved.Close()
	if err = fullNegative.FilterTableByNegation(allRemoved, []int{0, 1, 2}, []int{0, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if !allRemoved.Empty() {
		t.Fatal("nonempty full negative sieve must remove every target row")
	}
	emptyNegative, err := NewSieveRelation(signature, []bool{false, false, false})
	if err != nil {
		t.Fatal(err)
	}
	defer emptyNegative.Close()
	unchanged := newTarget()
	defer unchanged.Close()
	if err = emptyNegative.FilterTableByNegation(unchanged, []int{0}, []int{0}); err != nil || unchanged.NumRows() != 4 {
		t.Fatal("empty full negative changed target", unchanged.NumRows(), err)
	}
}

func TestSieveTableNegationZeroArityAndBounds(t *testing.T) {
	f := newTableRelationFixture(t)
	signature := sieveSignature(t)
	sieveTarget, err := NewSieveRelation(signature, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sieveTarget.Close()
	if _, err = sieveTarget.AddFact(Fact{}); err != nil {
		t.Fatal(err)
	}
	tableNegative, err := NewTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tableNegative.Close()
	if _, err = tableNegative.AddTableFact(Fact{}); err != nil {
		t.Fatal(err)
	}
	if err = sieveTarget.FilterByTableNegation(tableNegative, nil, nil); err != nil || !sieveTarget.Empty() {
		t.Fatal("zero-arity sieve target", sieveTarget.Empty(), err)
	}

	tableTarget, err := NewTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tableTarget.Close()
	if _, err = tableTarget.AddTableFact(Fact{}); err != nil {
		t.Fatal(err)
	}
	negativeSieve, err := NewFullSieve(signature)
	if err != nil {
		t.Fatal(err)
	}
	defer negativeSieve.Close()
	if err = negativeSieve.FilterTableByNegation(tableTarget, nil, nil); err != nil || !tableTarget.Empty() {
		t.Fatal("zero-arity table target", tableTarget.Empty(), err)
	}

	nonzero, err := NewSieveRelation(sieveSignature(t, 2), []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	defer nonzero.Close()
	booleanTable, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	defer booleanTable.Close()
	if err = nonzero.FilterByTableNegation(booleanTable, []int{1}, []int{0}); err == nil {
		t.Fatal("out-of-range target column accepted")
	}
	if err = nonzero.FilterTableByNegation(booleanTable, []int{0}, []int{1}); err == nil {
		t.Fatal("out-of-range negative column accepted")
	}
}
