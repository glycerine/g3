package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
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

func TestIntervalVectorColumnEquivalences(t *testing.T) {
	r := newIntervalColumns(3, false)
	point := func(n int64) smtcore.RelationInterval {
		v := big.NewRat(n, 1)
		return smtcore.NewRelationInterval(v, false, v, false)
	}
	astTestMust(t, r.intersect(0, smtcore.NewRelationInterval(big.NewRat(0, 1), false, big.NewRat(2, 1), false)))
	astTestMust(t, r.intersect(1, smtcore.NewRelationInterval(big.NewRat(1, 1), false, big.NewRat(3, 1), false)))
	astTestMust(t, r.equate(0, 1))
	if r.find(0) != 1 {
		t.Fatal("equal-size merge must select second root")
	}
	astTestMust(t, r.equate(1, 2))
	if r.find(2) != 1 {
		t.Fatal("merge must preserve the larger root")
	}
	astTestMust(t, r.intersect(2, point(2)))
	for i := uint32(0); i < 3; i++ {
		v := r.columns[r.find(i)]
		if !v.Contains(big.NewRat(2, 1)) || v.Contains(big.NewRat(1, 1)) {
			t.Fatalf("column %d did not share intersected range", i)
		}
	}
	astTestMust(t, r.intersect(1, point(3)))
	if !r.empty {
		t.Fatal("conflicting equal column must empty relation")
	}
	for i := uint32(0); i < 3; i++ {
		if r.find(i) != i {
			t.Fatal("set_empty must reset equivalences")
		}
	}

	r = newIntervalColumns(2, false)
	astTestMust(t, r.intersect(0, point(1)))
	astTestMust(t, r.intersect(1, point(2)))
	astTestMust(t, r.equate(0, 1))
	if !r.empty {
		t.Fatal("equating disjoint columns must empty relation")
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
