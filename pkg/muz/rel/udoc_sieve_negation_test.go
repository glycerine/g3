package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// TestUDocSieveNegationColumnClassification locks the intended
// sieve_relation_plugin::mk_filter_by_negation_fn semantics (AUDIT.md
// muz_rel_06-F5). z3 4.7.1 computes its "column is inner" flags as
// !is_inner_col, so it drops an equality between two inner columns (and
// then subtracts on zero columns, emptying the target) and turns an
// equality on a full negative column into the identity; see
// tst_z3_4_7_1_sieve_negation_inner_cols in src/test/z3_4_7_1_bugs.cpp.
// Every sieve here has columns (BV2, BV2) with column 0 full and column 1
// inner, as in that C++ test.
func TestUDocSieveNegationColumnClassification(t *testing.T) {
	factory, _, sort, values := udocTableUnionFixture(t)
	sorts := []ast.ASTHandle{sort, sort}
	sieve := func(inner ...int) *UDocSieve {
		t.Helper()
		relation, err := factory.NewEmpty([]ast.ASTHandle{sort})
		astTestMust(t, err)
		for _, v := range inner {
			_, err = relation.AddFact([]ast.ASTHandle{values[v]})
			astTestMust(t, err)
		}
		raw, err := AdoptUDocSieve(sorts, []bool{false, true}, relation)
		if err != nil {
			astTestMust(t, relation.Close())
		}
		result := ownUDocSieve(t, raw, err)
		assertUDocSieveMask(t, result, false, true)
		return result
	}

	// Both ends inner: {0,1} minus {1} joined on column 1 keeps only 0.
	target, negative := sieve(0, 1), sieve(1)
	astTestMust(t, target.FilterByNegation(negative, []uint32{1}, []uint32{1}))
	assertUDocSieveContains(t, target, true, values[2], values[0])
	assertUDocSieveContains(t, target, false, values[2], values[1])

	// Inner target column against a full negative column: the equality
	// always holds and is dropped, leaving a zero-column subtraction. The
	// table-backed sieve below empties the target; udoc's own
	// negation_filter_fn requires a joined column for a non-nullary negative
	// (SASSERT in udoc_relation.cpp), which g3 reports as ErrDOC.
	target, negative = sieve(0, 1), sieve(1)
	if err := target.FilterByNegation(negative, []uint32{1}, []uint32{0}); !errors.Is(err, ErrDOC) {
		t.Fatal("zero-column DOC negation", err)
	}

	// Full target column against an inner negative column: the identity.
	target, negative = sieve(0, 1), sieve(1)
	astTestMust(t, target.FilterByNegation(negative, []uint32{0}, []uint32{1}))
	assertUDocSieveContains(t, target, true, values[2], values[0])
	assertUDocSieveContains(t, target, true, values[2], values[1])

	// An empty negative removes nothing.
	target, negative = sieve(0, 1), sieve()
	astTestMust(t, target.FilterByNegation(negative, []uint32{1}, []uint32{1}))
	assertUDocSieveContains(t, target, true, values[2], values[0])
	assertUDocSieveContains(t, target, true, values[2], values[1])
}

// TestSieveNegationFullNegativeColumnSubtractsAll is the table-backed
// counterpart of tst_z3_4_7_1_sieve_negation_inner_cols' second case
// (AUDIT.md muz_rel_06-F5): an equality ending at a full negative column
// always holds, so a nonempty negative removes every target row. z3 4.7.1
// returns the identity instead. Both-inner joins, the full-target identity
// and the outer-column mapping are covered by TestSieveNegationFilterMappings
// and TestSieveNegationFilterUsesOuterColumnMappings.
func TestSieveNegationFullNegativeColumnSubtractsAll(t *testing.T) {
	signature := sieveSignature(t, 4, 4)
	target, err := NewSieveRelation(signature, []bool{false, true})
	if err != nil {
		t.Fatal(err)
	}
	negated, err := NewSieveRelation(signature, []bool{false, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, target, negated)
	for _, fact := range []Fact{{0, 0}, {0, 1}} {
		if _, err = target.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = negated.AddFact(Fact{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err = target.FilterByNegation(negated, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if !target.Empty() {
		t.Fatal("full negative column did not subtract the whole target")
	}
}
