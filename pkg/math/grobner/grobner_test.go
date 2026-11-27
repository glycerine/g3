package grobner

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	m    *ast.ASTManager
	s    *Solver
	sort ast.ASTHandle
}

func newFixture(t testing.TB) *fixture {
	t.Helper()
	b := arena.NewBudget(1 << 27)
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	must(t, err)
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	must(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	must(t, err)
	family, err := m.RegisterArithmeticPlugin()
	must(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	must(t, err)
	sortRef, err := m.NewRef(realSort)
	must(t, err)
	s, err := New(m)
	must(t, err)
	t.Cleanup(func() {
		must(t, s.Close())
		must(t, m.DelRef(&sortRef))
		must(t, m.Close())
		must(t, q.Close())
		must(t, sy.Close())
		if b.Used() != 0 {
			t.Errorf("owner graph retained %d bytes", b.Used())
		}
	})
	return &fixture{m: m, s: s, sort: realSort}
}

func (f *fixture) variable(t testing.TB, name string) ast.ASTHandle {
	t.Helper()
	n, err := f.m.Symbols.Intern(name)
	must(t, err)
	h, err := f.m.MkConst(n, f.sort)
	must(t, err)
	r, err := f.m.NewRef(h)
	must(t, err)
	t.Cleanup(func() { must(t, f.m.DelRef(&r)) })
	return h
}

func monomial(t testing.TB, s *Solver, n int64, vars ...ast.ASTHandle) *Monomial {
	t.Helper()
	c := numeral.Rational{Num: numeral.NewInt(int32(n)), Den: numeral.NewInt(1)}
	m, err := s.NewMonomial(&c, vars)
	must(t, err)
	return m
}

func TestBasisReducesNonlinearEquation(t *testing.T) {
	f := newFixture(t)
	x, y := f.variable(t, "x"), f.variable(t, "y")

	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x, y), monomial(t, f.s, -1)}, nil))
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x), monomial(t, f.s, -1)}, nil))
	done, err := f.s.ComputeBasis(100)
	must(t, err)
	if !done {
		t.Fatal("basis did not exhaust its worklist")
	}

	foundX, foundY := false, false
	for _, eq := range f.s.Equations() {
		if eq.NumMonomials() != 2 {
			continue
		}
		lead, constant := eq.Monomial(0), eq.Monomial(1)
		if lead.Degree() != 1 || constant.Degree() != 0 || !lead.Coefficient().IsOne() || !constant.Coefficient().IsMinusOne() {
			continue
		}
		if lead.Variable(0) == x {
			foundX = true
		}
		if lead.Variable(0) == y {
			foundY = true
		}
	}
	if !foundX || !foundY {
		t.Fatalf("reduced basis missing x-1 or y-1: x=%v y=%v", foundX, foundY)
	}
	if f.s.Stats.NumProcessed == 0 || f.s.Stats.Simplify == 0 {
		t.Fatal("basis statistics were not updated", f.s.Stats)
	}
}

func TestBasisSuperposesSharedLeadingFactor(t *testing.T) {
	f := newFixture(t)
	x, y, z := f.variable(t, "x"), f.variable(t, "y"), f.variable(t, "z")
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x, y), monomial(t, f.s, -1)}, nil))
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x, z), monomial(t, f.s, -1)}, nil))
	done, err := f.s.ComputeBasis(100)
	must(t, err)
	if !done || f.s.NumNewEquations() == 0 || f.s.Stats.Superpose == 0 {
		t.Fatal("shared factor did not produce a superposition", done, f.s.NumNewEquations(), f.s.Stats)
	}
	found := false
	for _, eq := range f.s.Equations() {
		if eq.NumMonomials() != 2 || eq.IsLinearCombination() {
			continue
		}
		a, b := eq.Monomial(0), eq.Monomial(1)
		if a.Degree() == 1 && b.Degree() == 1 && ((a.Variable(0) == y && b.Variable(0) == z) || (a.Variable(0) == z && b.Variable(0) == y)) {
			found = true
		}
	}
	if !found {
		t.Fatal("basis is missing the y-z superposition")
	}
}

func TestWeightsScopesDependenciesAndDisplay(t *testing.T) {
	f := newFixture(t)
	x, y := f.variable(t, "x"), f.variable(t, "y")
	must(t, f.s.SetWeight(y, 7))
	m := monomial(t, f.s, 2, x, y, x)
	if m.Variable(0) != y {
		t.Fatal("weighted variable did not sort first")
	}
	must(t, f.s.DelMonomial(m))

	leafX := DependencyLeaf(0)
	leafY := DependencyLeaf(1)
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x), monomial(t, f.s, -1)}, leafX))
	must(t, f.s.Push())
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, y), monomial(t, f.s, -1)}, leafY))
	if len(f.s.Equations()) != 2 || f.s.ScopeLevel() != 1 {
		t.Fatal("scope assertion was not recorded")
	}

	var out bytes.Buffer
	must(t, f.s.Display(&out))
	if out.Len() == 0 {
		t.Fatal("empty solver display")
	}
	must(t, f.s.Pop(1))
	if len(f.s.Equations()) != 1 || f.s.ScopeLevel() != 0 {
		t.Fatal("pop did not delete scoped equation")
	}
	if f.s.Equations()[0].Dependency() != leafX {
		t.Fatal("base dependency changed")
	}
}

func TestExpressionAssertionAndInconsistency(t *testing.T) {
	f := newFixture(t)
	x := f.variable(t, "x")
	two, err := f.m.ArithmeticRealNumeralText("2")
	must(t, err)
	one, err := f.m.ArithmeticRealNumeralText("1")
	must(t, err)
	twoX, err := f.m.MkArithmeticMul(two, x)
	must(t, err)
	lhs, err := f.m.MkArithmeticAdd(twoX, one)
	must(t, err)
	rhs, err := f.m.ArithmeticRealNumeralText("5")
	must(t, err)
	eq, err := f.m.MkEq(lhs, rhs)
	must(t, err)
	must(t, f.s.AssertEquality(eq, nil))
	if len(f.s.Equations()) != 1 {
		t.Fatal("expression equality was not asserted")
	}
	got := f.s.Equations()[0]
	if v, small := got.Monomial(1).Coefficient().SmallInt32(); !got.Monomial(0).Coefficient().IsOne() || !small || v != -2 {
		t.Fatal("equality was not normalized")
	}

	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x)}, nil))
	_, err = f.s.ComputeBasis(100)
	must(t, err)
	if !f.s.Inconsistent() {
		t.Fatal("inconsistent polynomial equations were not detected")
	}
	if err := f.s.Pop(1); err == nil {
		t.Fatal("invalid pop was accepted")
	}
}

func TestLaterScopeRewriteFreezesAndRestoresBaseEquation(t *testing.T) {
	f := newFixture(t)
	x, y := f.variable(t, "x"), f.variable(t, "y")
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x, y), monomial(t, f.s, -1)}, nil))
	done, err := f.s.ComputeBasis(100)
	must(t, err)
	if !done || len(f.s.Equations()) != 1 {
		t.Fatal("base equation was not processed")
	}
	base := f.s.Equations()[0]

	must(t, f.s.Push())
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x), monomial(t, f.s, -1)}, nil))
	done, err = f.s.ComputeBasis(100)
	must(t, err)
	if !done || len(f.s.Equations()) < 2 {
		t.Fatal("scoped rewrite did not produce its copied equation")
	}
	must(t, f.s.Pop(1))
	equations := f.s.Equations()
	if len(equations) != 1 || equations[0] != base || !base.live {
		t.Fatal("pop did not restore the frozen base equation")
	}
	if base.Monomial(0).Degree() != 2 {
		t.Fatal("base equation was destructively simplified across a scope")
	}
}

func TestDependencyJoinShape(t *testing.T) {
	a, b := DependencyLeaf(0), DependencyLeaf(1)
	if DependencyJoin(nil, a) != a || DependencyJoin(a, nil) != a || DependencyJoin(a, a) != a {
		t.Fatal("mk_join shortcuts")
	}
	left, right, joined := DependencyJoin(a, b).Children()
	if !joined || left != a || right != b || b.Leaf() != 1 {
		t.Fatal("join shape")
	}
	if _, _, joined = a.Children(); joined {
		t.Fatal("leaf reported as join")
	}
}

// math_misc_01-F1: grobner.cpp merge_monomials charges
// m_manager.limit().inc(sz) with the pre-merge monomial count, and returns
// before charging when sz == 0.
func TestMergeChargesResourceLimit(t *testing.T) {
	f := newFixture(t)
	x := f.variable(t, "x")
	before := f.m.ResourceLimit().Count()
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x), monomial(t, f.s, 2, x), monomial(t, f.s, 3, x), monomial(t, f.s, -1)}, nil))
	after := f.m.ResourceLimit().Count()
	if after-before != 4 {
		t.Fatalf("merge charged %d, want 4", after-before)
	}
	must(t, f.s.AssertMonomialsZero(nil, nil))
	if got := f.m.ResourceLimit().Count(); got != after {
		t.Fatalf("empty merge charged %d, want 0", got-after)
	}
}

// AUDIT.md math_misc_01-F2: equation_set is obj_hashtable<equation> hashed
// by m_bidx, so iteration follows core_hashtable slot order.
func TestEquationSetsIterateInHashSlotOrder(t *testing.T) {
	eqs := make([]*Equation, 16)
	for i := range eqs {
		eqs[i] = &Equation{index: uint32(i)}
	}
	order := func(s *equationSet) []uint32 {
		var r []uint32
		for _, e := range s.snapshot() {
			r = append(r, e.index)
		}
		return r
	}
	same := func(got, want []uint32) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	var s equationSet
	for _, i := range []int{3, 5, 9} {
		s.insert(eqs[i])
	}
	if got := order(&s); !same(got, []uint32{9, 3, 5}) {
		t.Fatalf("capacity-8 slot order = %v, want [9 3 5]", got)
	}
	var d equationSet
	for _, i := range []int{0, 1, 2} {
		d.insert(eqs[i])
	}
	d.erase(eqs[1])  // slot 2 is used, so slot 1 becomes deleted
	d.insert(eqs[9]) // probes the deleted slot 1 and reuses it
	if got := order(&d); !same(got, []uint32{0, 9, 2}) || d.numDeleted != 0 {
		t.Fatalf("deleted-slot reuse order = %v (deleted %d), want [0 9 2]", got, d.numDeleted)
	}
	var g equationSet
	for _, i := range []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0} {
		g.insert(eqs[i])
	}
	if len(g.table) != 16 {
		t.Fatalf("capacity = %d, want 16 after growth", len(g.table))
	}
	if got := order(&g); !same(got, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("grown slot order = %v", got)
	}
}

func TestUnmatchedRuleResourceCount(t *testing.T) {
	f := newFixture(t)
	x, y := f.variable(t, "x"), f.variable(t, "y")
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x)}, nil))
	must(t, f.s.Push())
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, y)}, nil))
	before := f.m.ResourceLimit().Count()
	e, changed, err := f.s.reduce(f.s.equations[1], f.s.equations[0])
	must(t, err)
	if e != f.s.equations[0] || changed || f.m.ResourceLimit().Count() != before {
		t.Fatalf("unmatched rule changed=%v, charged %d steps", changed, f.m.ResourceLimit().Count()-before)
	}
}

func TestScopedRuleReducesOlderEquation(t *testing.T) {
	f := newFixture(t)
	x, y := f.variable(t, "x"), f.variable(t, "y")
	baseDep := DependencyLeaf(0)
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x, y), monomial(t, f.s, -1)}, baseDep))
	done, err := f.s.ComputeBasis(100)
	must(t, err)
	if !done {
		t.Fatal("incomplete base")
	}
	base := f.s.Equations()[0]
	must(t, f.s.Push())
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x), monomial(t, f.s, -1)}, DependencyLeaf(1)))
	done, err = f.s.ComputeBasis(100)
	must(t, err)
	if !done {
		t.Fatal("incomplete scoped basis")
	}
	found := false
	for _, eq := range f.s.Equations() {
		if eq.NumMonomials() == 2 && eq.Monomial(0).Degree() == 1 && eq.Monomial(0).Variable(0) == y && eq.Monomial(1).Degree() == 0 && eq.Monomial(1).Coefficient().IsMinusOne() {
			found = true
		}
	}
	if !found {
		t.Fatal("scoped x=1 did not reduce x*y=1 to y=1")
	}
	must(t, f.s.Pop(1))
	eqs := f.s.Equations()
	if len(eqs) != 1 || eqs[0] != base || base.Monomial(0).Degree() != 2 || base.Dependency() != baseDep {
		t.Fatal("pop failed to restore original nonlinear equation")
	}
}

func TestScopedReductionCopiesPrefix(t *testing.T) {
	f := newFixture(t)
	x, y := f.variable(t, "x"), f.variable(t, "y")
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, y, y), monomial(t, f.s, 1, x)}, nil))
	base := f.s.equations[0]
	must(t, f.s.Push())
	must(t, f.s.AssertMonomialsZero([]*Monomial{monomial(t, f.s, 1, x)}, nil))
	reduced, changed, err := f.s.reduce(f.s.equations[1], base)
	must(t, err)
	if !changed || reduced == base || reduced.NumMonomials() != 1 || reduced.Monomial(0).Degree() != 2 || reduced.Monomial(0) == base.Monomial(0) || base.NumMonomials() != 2 {
		t.Fatalf("scoped x=0 substitution changed=%v, original size=%d, reduced size=%d", changed, base.NumMonomials(), reduced.NumMonomials())
	}
}

// A processed rule is tried against each target polynomial. Most pairs
// cannot reduce each other, even when their leading degrees agree.
func BenchmarkUnmatchedGrobnerRule(b *testing.B) {
	f := newFixture(b)
	x, y, z := f.variable(b, "x"), f.variable(b, "y"), f.variable(b, "z")
	for _, vars := range [][]ast.ASTHandle{{x}, {x, y}, {z}, {y, z}} {
		must(b, f.s.AssertMonomialsZero([]*Monomial{monomial(b, f.s, 1, vars...), monomial(b, f.s, -1)}, nil))
	}
	for _, tc := range []struct {
		name           string
		source, target int
	}{
		{"Linear", 0, 2}, {"Quadratic", 1, 3}, {"LinearAgainstQuadratic", 0, 3},
	} {
		b.Run(tc.name, func(b *testing.B) {
			source, target := f.s.equations[tc.source], f.s.equations[tc.target]
			b.ReportAllocs()
			for b.Loop() {
				e, changed, err := f.s.reduce(source, target)
				if err != nil || changed || e != target {
					b.Fatalf("unexpected reduction: changed=%v, error=%v", changed, err)
				}
			}
		})
	}
}

func TestLinearFactorRemainder(t *testing.T) {
	f := newFixture(t)
	x, y, z := f.variable(t, "x"), f.variable(t, "y"), f.variable(t, "z")
	product := monomial(t, f.s, 1, x, y, y, z)
	defer func() { must(t, f.s.DelMonomial(product)) }()
	vars := product.vars
	for _, idx := range []int{0, 1, 3} {
		factor := monomial(t, f.s, 1, vars[idx])
		matched, rest, err := f.s.subset(factor, product)
		must(t, err)
		must(t, f.s.DelMonomial(factor))
		want := append([]ast.ASTHandle(nil), vars[:idx]...)
		want = append(want, vars[idx+1:]...)
		if !matched || !reflect.DeepEqual(rest, want) {
			t.Fatalf("factor %d: got %v, %v; want %v", idx, rest, matched, want)
		}
	}
}
