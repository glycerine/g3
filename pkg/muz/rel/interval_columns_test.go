package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalColumnsInterpretedFilters(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle { astTestMust(t, err); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	zero, two := own(m.ArithmeticRealNumeralText("0")), own(m.ArithmeticRealNumeralText("2"))
	r := newIntervalColumns(2, false)
	astTestMust(t, r.filterInterpreted(m, own(m.MkArithmeticGE(x, zero))))
	astTestMust(t, r.filterInterpreted(m, own(m.MkArithmeticLT(x, two))))
	astTestMust(t, r.filterInterpreted(m, own(m.MkEq(y, x))))
	for i, v := range r.columns {
		lo, loOpen, hi, hiOpen := v.Bounds()
		if lo == nil || hi == nil || lo.RatString() != "0" || hi.RatString() != "2" || loOpen || !hiOpen {
			t.Fatalf("column %d: %v %v %v %v", i, lo, loOpen, hi, hiOpen)
		}
	}
	astTestMust(t, r.filterInterpreted(m, own(m.MkArithmeticGE(y, two))))
	if !r.empty {
		t.Fatal("disjoint bounds must empty the relation")
	}
	falseRelation := newIntervalColumns(1, false)
	astTestMust(t, falseRelation.filterInterpreted(m, own(m.MkFalse())))
	if !falseRelation.empty {
		t.Fatal("false condition must empty the relation")
	}
}

func TestIntervalColumnsBinaryPropagation(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle { astTestMust(t, err); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	one, five := own(m.ArithmeticRealNumeralText("1")), own(m.ArithmeticRealNumeralText("5"))
	r := newIntervalColumns(2, false)
	astTestMust(t, r.filterInterpreted(m, own(m.MkArithmeticLE(x, five))))
	astTestMust(t, r.filterInterpreted(m, own(m.MkArithmeticGE(y, one))))
	astTestMust(t, r.filterInterpreted(m, own(m.MkArithmeticLT(y, x))))
	lo, loOpen, _, _ := r.columns[0].Bounds()
	_, _, hi, hiOpen := r.columns[1].Bounds()
	if lo == nil || lo.RatString() != "1" || !loOpen || hi == nil || hi.RatString() != "5" || !hiOpen {
		t.Fatalf("binary propagation: x lower %v open=%v, y upper %v open=%v", lo, loOpen, hi, hiOpen)
	}
}
