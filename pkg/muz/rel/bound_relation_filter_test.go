package rel

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoundRelationInterpretedVariableComparisons(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	less, le, greater, ge := own(m.MkArithmeticLT(x, y)), own(m.MkArithmeticLE(x, y)), own(m.MkArithmeticGT(x, y)), own(m.MkArithmeticGE(x, y))
	for _, tc := range []struct {
		name    string
		formula ast.ASTHandle
		left    uint32
		strict  bool
	}{
		{"lt", less, 0, true}, {"le", le, 0, false}, {"gt", greater, 1, true}, {"ge", ge, 1, false},
		{"not lt", own(m.MkNot(less)), 1, false}, {"not le", own(m.MkNot(le)), 1, true},
		{"not gt", own(m.MkNot(greater)), 0, false}, {"not ge", own(m.MkNot(ge)), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewBoundRelation(2, false)
			ok, err := r.FilterInterpreted(m, tc.formula)
			astTestMust(t, err)
			if !ok {
				t.Fatal("comparison not handled")
			}
			strict, weak, err := r.Bounds(tc.left)
			astTestMust(t, err)
			want := []uint32{1 - tc.left}
			if tc.strict && !reflect.DeepEqual(strict, want) || !tc.strict && !reflect.DeepEqual(weak, want) {
				t.Fatalf("bounds: %v %v", strict, weak)
			}
		})
	}
	r := NewBoundRelation(2, false)
	constant := own(m.ArithmeticRealNumeralText("1"))
	unknown := own(m.MkArithmeticLT(x, constant))
	ok, err := r.FilterInterpreted(m, unknown)
	astTestMust(t, err)
	if ok {
		t.Fatal("non-variable comparison should be ignored")
	}
	astTestMust(t, r.AddLess(0, 1))
	ok, err = r.FilterInterpreted(m, own(m.MkFalse()))
	astTestMust(t, err)
	strict, weak, err := r.Bounds(0)
	astTestMust(t, err)
	if !ok || !r.Empty() || len(strict) != 0 || len(weak) != 0 {
		t.Fatal("false filter must reset relation")
	}
}

func TestBoundRelationInterpretedPositiveOffsetEquality(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, integer)), own(m.MkVar(1, integer))
	positive, zero := own(m.ArithmeticIntegerNumeralText("2")), own(m.ArithmeticIntegerNumeralText("0"))
	for _, sum := range []ast.ASTHandle{own(m.MkArithmeticAdd(positive, y)), own(m.MkArithmeticAdd(y, positive))} {
		r := NewBoundRelation(2, false)
		ok, err := r.FilterInterpreted(m, own(m.MkEq(x, sum)))
		astTestMust(t, err)
		strict, _, err := r.Bounds(1)
		astTestMust(t, err)
		if !ok || !reflect.DeepEqual(strict, []uint32{0}) {
			t.Fatal("positive offset", ok, strict)
		}
	}
	r := NewBoundRelation(2, false)
	ok, err := r.FilterInterpreted(m, own(m.MkEq(x, own(m.MkArithmeticAdd(y, zero)))))
	astTestMust(t, err)
	if ok {
		t.Fatal("zero offset is not a strict bound")
	}
}
