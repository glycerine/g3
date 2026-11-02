package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationSignatureCapability(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)

	for _, tc := range []struct {
		name  string
		sorts []ast.ASTHandle
		want  bool
	}{
		{name: "empty", want: true},
		{name: "integer-real", sorts: []ast.ASTHandle{integer, real}, want: true},
		{name: "boolean", sorts: []ast.ASTHandle{integer, boolean}},
		{name: "expression", sorts: []ast.ASTHandle{zero}},
		{name: "invalid", sorts: []ast.ASTHandle{{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanHandleIntervalSorts(m, tc.sorts); got != tc.want {
				t.Fatalf("capability=%v, want %v", got, tc.want)
			}
		})
	}
	if CanHandleIntervalSorts(nil, []ast.ASTHandle{integer}) {
		t.Fatal("nil manager accepted")
	}
}

func TestIntervalRelationNaryIdenticalFilter(t *testing.T) {
	n := func(value int64) *big.Rat { return big.NewRat(value, 1) }
	r := NewIntervalEmpty(4)
	for _, tuple := range [][]int64{{0, 2, 4, 6}, {10, 8, 6, 4}} {
		fact := make([]*big.Rat, len(tuple))
		for i, value := range tuple {
			fact[i] = n(value)
		}
		if err := r.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.FilterIdenticalColumns([]uint32{0, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if !r.ContainsFact([]*big.Rat{n(5), n(3), n(5), n(5)}) {
		t.Fatal("n-ary filter did not share the intersection")
	}
	if r.ContainsFact([]*big.Rat{n(5), n(3), n(5), n(4)}) {
		t.Fatal("n-ary filter left a listed column independent")
	}

	before, err := r.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err = r.FilterIdenticalColumns([]uint32{0, 1, 9}); err == nil {
		t.Fatal("out-of-range column accepted")
	}
	if got, want := r.ContainsFact([]*big.Rat{n(5), n(3), n(5), n(5)}), before.ContainsFact([]*big.Rat{n(5), n(3), n(5), n(5)}); got != want {
		t.Fatal("failed validation partially mutated relation")
	}
	if err = r.FilterIdenticalColumns(nil); err != nil {
		t.Fatal("empty source filter must be a no-op", err)
	}
	if err = r.FilterIdenticalColumns([]uint32{99}); err != nil {
		t.Fatal("singleton source filter must be a no-op", err)
	}
	if err = (*IntervalRelation)(nil).FilterIdenticalColumns(nil); err == nil {
		t.Fatal("nil relation accepted")
	}
}

func TestIntervalRelationASTEqualFilter(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	value, err := m.ArithmeticRealNumeralText("3/2")
	astTestMust(t, err)
	astTestPin(t, m, value)
	r := NewIntervalFull(1)
	if err = r.FilterEqualExpression(m, 0, value); err != nil {
		t.Fatal(err)
	}
	if !r.ContainsFact([]*big.Rat{big.NewRat(3, 2)}) || r.ContainsFact([]*big.Rat{big.NewRat(1, 1)}) {
		t.Fatal("AST numeral filter did not capture the exact rational")
	}

	variable, err := m.MkVar(0, real)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	if err = r.FilterEqualExpression(m, 0, variable); err == nil {
		t.Fatal("non-numeral equality value accepted")
	}
	if !r.ContainsFact([]*big.Rat{big.NewRat(3, 2)}) {
		t.Fatal("rejected value changed the relation")
	}
	if err = r.FilterEqualExpression(m, 1, value); err == nil {
		t.Fatal("out-of-range equality column accepted")
	}
	if err = r.FilterEqualExpression(nil, 0, value); err == nil {
		t.Fatal("nil manager accepted")
	}
	if err = (*IntervalRelation)(nil).FilterEqualExpression(m, 0, value); err == nil {
		t.Fatal("nil relation accepted")
	}
}
