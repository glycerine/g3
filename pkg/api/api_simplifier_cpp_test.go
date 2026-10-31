package api

// Source: src/test/simplifier.cpp test_bv and test_bool.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifierSourceBitVectorConstants(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	b1 := f.pin(f.c.BitVectorSort(1))
	b2 := f.pin(f.c.BitVectorSort(2))
	wide := f.pin(f.c.BitVectorSort(72))
	one := f.pin(f.c.BitVectorValue(1, b1))
	three := f.pin(f.c.BitVectorValue(3, b2))
	twelve := f.pin(f.c.BitVectorValue(12, wide))
	thirteen := f.pin(f.c.BitVectorValue(13, wide))
	extended := f.pin(f.c.BitVectorSignExtend(1, one))
	equal := f.pin(f.c.Equal(three, extended))
	ref, err := f.c.Simplify(equal)
	astTestMust(t, err)
	v, err := f.m.RefGet(ref)
	astTestMust(t, err)
	if !f.m.IsTrue(v) {
		t.Fatal("one-bit sign extension")
	}
	astTestMust(t, f.m.DelRef(&ref))
	firstNot := f.pin(f.c.BitVectorNot(twelve))
	ref, err = f.c.Simplify(firstNot)
	astTestMust(t, err)
	notValue, err := f.m.RefGet(ref)
	astTestMust(t, err)
	again := f.pin(f.c.BitVectorNot(notValue))
	astTestMust(t, f.m.DelRef(&ref))
	terms := []struct {
		name    string
		term    ast.ASTHandle
		boolean bool
	}{
		{"not", firstNot, false}, {"double-not", again, false}, {"and", f.pin(f.c.BitVectorAnd(twelve, thirteen)), false},
		{"or", f.pin(f.c.BitVectorOr(twelve, thirteen)), false}, {"xor", f.pin(f.c.BitVectorXor(twelve, thirteen)), false},
		{"nand", f.pin(f.c.BitVectorNand(twelve, thirteen)), false}, {"nor", f.pin(f.c.BitVectorNor(twelve, thirteen)), false}, {"xnor", f.pin(f.c.BitVectorXnor(twelve, thirteen)), false},
		{"neg", f.pin(f.c.BitVectorNeg(twelve)), false}, {"add", f.pin(f.c.BitVectorAdd(twelve, thirteen)), false}, {"sub", f.pin(f.c.BitVectorSub(twelve, thirteen)), false}, {"mul", f.pin(f.c.BitVectorMul(twelve, thirteen)), false},
		{"udiv", f.pin(f.c.BitVectorUDiv(twelve, thirteen)), false}, {"sdiv", f.pin(f.c.BitVectorSDiv(twelve, thirteen)), false}, {"srem", f.pin(f.c.BitVectorSRem(twelve, thirteen)), false},
		{"uge", f.pin(f.c.BitVectorUGE(twelve, thirteen)), true}, {"sge", f.pin(f.c.BitVectorSGE(twelve, thirteen)), true}, {"ugt", f.pin(f.c.BitVectorUGT(twelve, thirteen)), true}, {"sgt", f.pin(f.c.BitVectorSGT(twelve, thirteen)), true},
		{"ule", f.pin(f.c.BitVectorULE(twelve, thirteen)), true}, {"ult", f.pin(f.c.BitVectorULT(twelve, thirteen)), true}, {"sle", f.pin(f.c.BitVectorSLE(twelve, thirteen)), true}, {"slt", f.pin(f.c.BitVectorSLT(twelve, thirteen)), true},
		{"concat", f.pin(f.c.BitVectorConcat(twelve, thirteen)), false}, {"extract", f.pin(f.c.BitVectorExtract(43, 1, thirteen)), false},
		{"sign-ext", f.pin(f.c.BitVectorSignExtend(33, thirteen)), false}, {"zero-ext", f.pin(f.c.BitVectorZeroExtend(33, thirteen)), false},
		{"shl", f.pin(f.c.BitVectorSHL(twelve, thirteen)), false}, {"lshr", f.pin(f.c.BitVectorLSHR(twelve, thirteen)), false}, {"ashr", f.pin(f.c.BitVectorASHR(twelve, thirteen)), false},
		{"rotate-left", f.pin(f.c.BitVectorRotateLeft(21, thirteen)), false}, {"rotate-right", f.pin(f.c.BitVectorRotateRight(21, thirteen)), false},
	}
	for _, tc := range terms {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := f.c.Simplify(tc.term)
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&ref)) }()
			v, err := f.m.RefGet(ref)
			astTestMust(t, err)
			if tc.boolean {
				b, err := f.c.BooleanValue(v)
				astTestMust(t, err)
				if b == lbool.Undef {
					t.Fatal("comparison did not reduce to a Boolean constant")
				}
			} else {
				ok, err := f.c.IsNumeral(v)
				astTestMust(t, err)
				if !ok {
					t.Fatal("bit-vector expression did not reduce to a numeral")
				}
			}
		})
	}
}

func TestSimplifierSourceBooleanConstants(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	yes := f.pin(f.m.MkTrue())
	no := f.pin(f.m.MkFalse())
	for _, form := range []ast.ASTHandle{f.pin(f.c.Not(f.pin(f.c.Equal(no, yes)))), f.pin(f.c.Not(f.pin(f.c.Iff(no, yes))))} {
		r, err := f.c.Simplify(form)
		astTestMust(t, err)
		h, err := f.m.RefGet(r)
		astTestMust(t, err)
		if !f.m.IsTrue(h) {
			t.Fatal("Boolean simplification")
		}
		astTestMust(t, f.m.DelRef(&r))
	}
}

// Source: src/test/simplifier.cpp test_datatypes.
func TestSimplifierSourceListDatatype(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.IntegerSort())
	list, err := f.c.ListSort("int_list", sort)
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{list.Sort, list.Nil, list.IsNil, list.Cons, list.IsCons, list.Head, list.Tail} {
		f.pin(h, nil)
	}
	nilValue := f.pin(f.c.Apply(list.Nil))
	one := f.pin(f.c.IntNumeral(1, sort))
	cons := f.pin(f.c.Apply(list.Cons, one, nilValue))
	yes := f.pin(f.m.MkTrue())
	no := f.pin(f.m.MkFalse())
	for _, tc := range []struct{ input, want ast.ASTHandle }{
		{f.pin(f.c.Apply(list.IsNil, nilValue)), yes}, {f.pin(f.c.Apply(list.IsCons, nilValue)), no},
		{f.pin(f.c.Apply(list.Tail, cons)), nilValue}, {f.pin(f.c.Apply(list.Head, cons)), one}, {f.pin(f.c.Equal(nilValue, cons)), no},
	} {
		ref, err := f.c.Simplify(tc.input)
		astTestMust(t, err)
		got, err := f.m.RefGet(ref)
		astTestMust(t, err)
		if got != tc.want {
			t.Fatal("list simplification", got, tc.want)
		}
		astTestMust(t, f.m.DelRef(&ref))
	}
}

// Source: src/test/simplifier.cpp test_array and test_skolemize_bug.
func TestSimplifierSourceArraysAndQuantifier(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	integer := f.pin(f.c.IntegerSort())
	array := f.pin(f.c.ArraySort([]ast.ASTHandle{integer}, integer))
	nums := make([]ast.ASTHandle, 5)
	for i := 1; i <= 4; i++ {
		nums[i] = f.pin(f.c.IntNumeral(int64(i), integer))
	}
	sym := f.pin(f.c.Constant("s1", integer))
	base := f.pin(f.c.ConstantArray(array, nums[1]))
	store := func(a, i, v ast.ASTHandle) ast.ASTHandle { return f.pin(f.c.ArrayStore(a, []ast.ASTHandle{i}, v)) }
	x1 := store(store(base, nums[2], nums[3]), nums[1], nums[4])
	x2 := store(store(base, nums[1], nums[4]), nums[2], nums[3])
	x3 := store(store(base, sym, nums[1]), nums[2], nums[3])
	x4 := store(x1, nums[2], nums[3])
	values := []ast.ASTHandle{x1, x2, x3, x4}
	for _, a := range values {
		for _, b := range values {
			ref, err := f.c.Simplify(f.pin(f.c.Equal(a, b)))
			astTestMust(t, err)
			astTestMust(t, f.m.DelRef(&ref))
		}
	}
	for _, tc := range []struct{ index, want ast.ASTHandle }{{nums[1], nums[4]}, {nums[4], nums[1]}} {
		sel := f.pin(f.c.ArraySelect(x1, tc.index))
		ref, err := f.c.Simplify(sel)
		astTestMust(t, err)
		v, err := f.m.RefGet(ref)
		astTestMust(t, err)
		if v != tc.want {
			t.Fatal("array selection", v, tc.want)
		}
		astTestMust(t, f.m.DelRef(&ref))
	}
	real := f.pin(f.c.RealSort())
	x := f.pin(f.c.BoundVariable(0, real))
	y := f.pin(f.c.Constant("y", real))
	xp := f.pin(f.c.Constant("xp", real))
	zero := f.pin(f.c.IntNumeral(0, real))
	one := f.pin(f.c.IntNumeral(1, real))
	body := f.pin(f.c.And(f.pin(f.c.Equal(f.pin(f.c.ArithmeticAdd(x, one)), xp)), f.pin(f.c.ArithmeticGE(f.pin(f.c.ArithmeticAdd(x, y)), zero))))
	quantified := f.pin(f.c.Exists([]ast.ASTHandle{real}, []string{"x"}, body))
	ref, err := f.c.Simplify(quantified)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&ref))
}
