package api

// Source: src/test/api.cpp Windows API driver.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISourceApplicationsAndBitVectors(t *testing.T) {
	t.Run("applications", func(t *testing.T) {
		f := newSMTAPIFixture(t, "ALL", nil)
		sort := f.pin(f.c.UninterpretedSort("A"))
		decl := f.pin(f.c.Function("f", []ast.ASTHandle{sort}, sort))
		x := f.pin(f.c.Constant("x", sort))
		result := x
		for i := 0; i < 5; i++ {
			result = f.pin(f.c.Apply(decl, result))
		}
		formula := f.pin(f.c.Not(f.pin(f.c.Equal(x, result))))
		solver, err := f.c.NewSimpleSolver()
		astTestMust(t, err)
		defer func() { astTestMust(t, solver.Close()) }()
		astTestMust(t, solver.Assert(formula))
		status, err := solver.Check()
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("nested applications", status)
		}
	})
	t.Run("bitvectors", func(t *testing.T) {
		f := newSMTAPIFixture(t, "ALL", nil)
		solver, err := f.c.NewSimpleSolver()
		astTestMust(t, err)
		defer func() { astTestMust(t, solver.Close()) }()
		for _, width := range []uint32{30, 31, 32} {
			sort := f.pin(f.c.BitVectorSort(width))
			x := f.pin(f.c.FreshConstant("x", sort))
			negative := f.pin(f.c.IntNumeral(-1, sort))
			zero := f.pin(f.c.IntNumeral(0, sort))
			formula := f.pin(f.c.Equal(negative, f.pin(f.c.BitVectorAdd(zero, x))))
			astTestMust(t, solver.Assert(formula))
			status, err := solver.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("bit-vector width", width, status)
			}
		}
	})
}

func TestAPISourceDistinctRejectsMismatchedSorts(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	bv8 := f.pin(f.c.BitVectorSort(8))
	bv32 := f.pin(f.c.BitVectorSort(32))
	zero8 := f.pin(f.c.IntNumeral(0, bv8))
	zero32 := f.pin(f.c.IntNumeral(0, bv32))
	if _, err := f.c.Distinct(zero8, zero32); err == nil {
		t.Fatal("distinct accepted incompatible bit-vector sorts")
	}
}
