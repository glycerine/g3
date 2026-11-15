package api

// Z3_get_implied_equalities class identifiers (AUDIT.md smtcore_03-F10).
// The expected identifiers were produced by the C++ z3 4.7.1 release
// library (build/libz3.a) through a C harness that builds the same terms
// with Z3_mk_simple_solver and calls Z3_get_implied_equalities; they follow
// get_implied_equalities_impl's union_find merge rule, its model-based value
// buckets, filter_basic and the array extensionality branch.

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// newImpliedEqualityFixture builds a context without set-logic, as the C API
// context of Z3_mk_simple_solver has no logic.
func newImpliedEqualityFixture(t *testing.T) (*smtAPIFixture, *APISimpleSolver) {
	t.Helper()
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	m, err := c.Manager()
	astTestMust(t, err)
	f := &smtAPIFixture{t: t, c: c, m: m}
	s, err := c.NewSimpleSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	return f, s
}

func checkImpliedEqualityIDs(t *testing.T, s *APISimpleSolver, terms []ast.ASTHandle, wantStatus lbool.LBool, want []uint32) {
	t.Helper()
	status, classes, err := s.ImpliedEqualities(terms)
	astTestMust(t, err)
	if status != wantStatus || !reflect.DeepEqual(classes, want) {
		t.Fatalf("implied equalities: got %v %v, C++ gives %v %v", status, classes, wantStatus, want)
	}
}

func TestImpliedEqualitiesCPPClassIDsUninterpreted(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	u := f.pin(f.c.UninterpretedSort("U"))
	boolean := f.pin(f.c.BoolSort())
	a := f.pin(f.c.Constant("a", u))
	b := f.pin(f.c.Constant("b", u))
	c := f.pin(f.c.Constant("c", u))
	d := f.pin(f.c.Constant("d", u))
	p := f.pin(f.c.Constant("p", boolean))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(a, b))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(b, c))))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{a, b, c, d, p}, lbool.True, []uint32{1, 1, 1, 3, 4})
	e := f.pin(f.c.Constant("e", u))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(d, e))))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{d, a, e, b, p, c}, lbool.True, []uint32{2, 3, 2, 3, 4, 3})
}

func TestImpliedEqualitiesCPPClassIDsArithmetic(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	intSort := f.pin(f.c.IntegerSort())
	a := f.pin(f.c.Constant("a", intSort))
	b := f.pin(f.c.Constant("b", intSort))
	c := f.pin(f.c.Constant("c", intSort))
	d := f.pin(f.c.Constant("d", intSort))
	decl := f.pin(f.c.Function("f", []ast.ASTHandle{intSort}, intSort))
	fa := f.pin(f.c.Apply(decl, a))
	fb := f.pin(f.c.Apply(decl, b))
	fc := f.pin(f.c.Apply(decl, c))
	terms := []ast.ASTHandle{a, b, c, d, fa, fb, fc}
	astTestMust(t, s.Assert(f.pin(f.c.Equal(a, b))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(b, d))))
	astTestMust(t, s.Assert(f.pin(f.c.ArithmeticLE(fa, fc))))
	astTestMust(t, s.Assert(f.pin(f.c.ArithmeticLE(fc, d))))
	checkImpliedEqualityIDs(t, s, terms, lbool.True, []uint32{0, 0, 2, 0, 4, 4, 6})
	astTestMust(t, s.Assert(f.pin(f.c.ArithmeticLE(b, fa))))
	checkImpliedEqualityIDs(t, s, terms, lbool.True, []uint32{0, 0, 2, 0, 0, 0, 0})
}

func TestImpliedEqualitiesCPPClassIDsArrays(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	intSort := f.pin(f.c.IntegerSort())
	a := f.pin(f.c.Constant("a", intSort))
	b := f.pin(f.c.Constant("b", intSort))
	one := f.pin(f.c.IntNumeral(1, intSort))
	two := f.pin(f.c.IntNumeral(2, intSort))
	arraySort := f.pin(f.c.ArraySort([]ast.ASTHandle{intSort}, intSort))
	x := f.pin(f.c.ConstantArray(arraySort, one))
	y := f.pin(f.c.ArrayStore(x, []ast.ASTHandle{one}, a))
	z := f.pin(f.c.ArrayStore(y, []ast.ASTHandle{two}, b))
	u := f.pin(f.c.ArrayStore(x, []ast.ASTHandle{two}, b))
	v := f.pin(f.c.ArrayStore(u, []ast.ASTHandle{one}, a))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{x, y, z, u, v}, lbool.True, []uint32{0, 1, 2, 3, 2})
}

// The first check is unknown (nonlinear real arithmetic); C++ still fills
// the class identifiers because it only skips them on l_false.
func TestImpliedEqualitiesCPPClassIDsUnknown(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	real := f.pin(f.c.RealSort())
	x := f.pin(f.c.Constant("x", real))
	y := f.pin(f.c.Constant("y", real))
	z := f.pin(f.c.Constant("z", real))
	square := f.pin(f.c.ArithmeticMul(x, x))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(square, f.pin(f.c.Numeral("2", real))))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(y, x))))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{z, x, y}, lbool.Undef, []uint32{0, 1, 1})
}

func TestImpliedEqualitiesCPPClassIDsUnsat(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	p := f.pin(f.c.Constant("p", f.pin(f.c.BoolSort())))
	astTestMust(t, s.Assert(p))
	astTestMust(t, s.Assert(f.pin(f.c.Not(p))))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{p}, lbool.False, nil)
}

func TestImpliedEqualitiesCPPClassIDsMixedValueSorts(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	boolean := f.pin(f.c.BoolSort())
	real := f.pin(f.c.RealSort())
	bv := f.pin(f.c.BitVectorSort(8))
	p := f.pin(f.c.Constant("p", boolean))
	q := f.pin(f.c.Constant("q", boolean))
	r := f.pin(f.c.Constant("r", boolean))
	x := f.pin(f.c.Constant("x", real))
	y := f.pin(f.c.Constant("y", real))
	w := f.pin(f.c.Constant("w", real))
	u := f.pin(f.c.Constant("u", bv))
	v := f.pin(f.c.Constant("v", bv))
	oneBV := f.pin(f.c.BitVectorValue(1, bv))
	astTestMust(t, s.Assert(f.pin(f.c.Iff(p, q))))
	astTestMust(t, s.Assert(f.pin(f.c.Not(r))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(f.pin(f.c.ArithmeticAdd(x, y)), w))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(y, f.pin(f.c.Real(0, 1))))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(f.pin(f.c.BitVectorAdd(u, oneBV)), v))))
	vMinusOne := f.pin(f.c.BitVectorSub(v, oneBV))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{u, r, x, p, w, v, q, y, vMinusOne}, lbool.True, []uint32{0, 1, 2, 3, 2, 5, 3, 7, 0})
}

// Arrays with an uninterpreted range: the array branch's selects on a fresh
// index are not of a value sort, so they go through filter_basic.
func TestImpliedEqualitiesCPPClassIDsArraysOfUninterpreted(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	intSort := f.pin(f.c.IntegerSort())
	u := f.pin(f.c.UninterpretedSort("U"))
	arraySort := f.pin(f.c.ArraySort([]ast.ASTHandle{intSort}, u))
	a1 := f.pin(f.c.Constant("a1", arraySort))
	a2 := f.pin(f.c.Constant("a2", arraySort))
	a3 := f.pin(f.c.Constant("a3", arraySort))
	u0 := f.pin(f.c.Constant("u0", u))
	zero := f.pin(f.c.IntNumeral(0, intSort))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(a1, a2))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(a3, f.pin(f.c.ArrayStore(a1, []ast.ASTHandle{zero}, u0))))))
	stored := f.pin(f.c.ArrayStore(a2, []ast.ASTHandle{zero}, u0))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{a3, a1, a2, stored}, lbool.True, []uint32{3, 2, 2, 3})
}

// A datatype whose fields are value sorts is a value sort (is_value_sort).
func TestImpliedEqualitiesCPPClassIDsTuples(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	intSort := f.pin(f.c.IntegerSort())
	boolean := f.pin(f.c.BoolSort())
	pair, err := f.c.TupleSort("Pair", []string{"first", "second"}, []ast.ASTHandle{intSort, boolean})
	astTestMust(t, err)
	t1 := f.pin(f.c.Constant("t1", pair.Sort))
	t2 := f.pin(f.c.Constant("t2", pair.Sort))
	t3 := f.pin(f.c.Constant("t3", pair.Sort))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(t2, t1))))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{t3, t1, t2}, lbool.True, []uint32{0, 1, 1})
}

// Z3_mk_solver: combined_solver answers through its incremental smt solver
// after the procedure's push, and that solver builds the model.
func TestImpliedEqualitiesCPPClassIDsCombinedSolver(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	m, err := c.Manager()
	astTestMust(t, err)
	f := &smtAPIFixture{t: t, c: c, m: m}
	s, err := c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	intSort := f.pin(f.c.IntegerSort())
	a := f.pin(f.c.Constant("a", intSort))
	b := f.pin(f.c.Constant("b", intSort))
	cc := f.pin(f.c.Constant("c", intSort))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(cc, a))))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{a, b, cc}, lbool.True, []uint32{0, 1, 0})
}
