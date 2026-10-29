package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFloatingPointSortsAndShapes(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	tests := []struct {
		name        string
		make        func() (ast.ASTHandle, error)
		exponent    uint32
		significand uint32
	}{
		{"half", f.c.FloatingPointHalfSort, 5, 11},
		{"single", f.c.FloatingPointSingleSort, 8, 24},
		{"double", f.c.FloatingPointDoubleSort, 11, 53},
		{"quadruple", f.c.FloatingPointQuadrupleSort, 15, 113},
		{"custom", func() (ast.ASTHandle, error) { return f.c.FloatingPointSort(6, 19) }, 6, 19},
	}
	for _, test := range tests {
		sort := f.pin(test.make())
		exponent, significand, err := f.c.FloatingPointSortShape(sort)
		if err != nil || exponent != test.exponent || significand != test.significand {
			t.Fatalf("%s shape: (%d,%d), %v", test.name, exponent, significand, err)
		}
	}
	rounding := f.pin(f.c.FloatingPointRoundingModeSort())
	if matched, err := f.m.IsRoundingModeSort(rounding); err != nil || !matched {
		t.Fatal("rounding-mode sort", matched, err)
	}
	if _, err := f.c.FloatingPointSort(1, 10); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("short exponent", err)
	}
	if _, err := f.c.FloatingPointSort(5, 2); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("short significand", err)
	}
	if _, _, err := f.c.FloatingPointSortShape(f.pin(f.c.BoolSort())); err == nil {
		t.Fatal("Boolean accepted as floating-point sort")
	}
}
