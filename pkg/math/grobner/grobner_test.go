package grobner

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func must(t *testing.T, err error) {
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

func newFixture(t *testing.T) *fixture {
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

func (f *fixture) variable(t *testing.T, name string) ast.ASTHandle {
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

func monomial(t *testing.T, s *Solver, n int64, vars ...ast.ASTHandle) *Monomial {
	t.Helper()
	m, err := s.NewMonomial(big.NewRat(n, 1), vars)
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
		if lead.Degree() != 1 || constant.Degree() != 0 || lead.Coefficient().Cmp(big.NewRat(1, 1)) != 0 || constant.Coefficient().Cmp(big.NewRat(-1, 1)) != 0 {
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
	if got.Monomial(0).Coefficient().Cmp(big.NewRat(1, 1)) != 0 || got.Monomial(1).Coefficient().Cmp(big.NewRat(-2, 1)) != 0 {
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
