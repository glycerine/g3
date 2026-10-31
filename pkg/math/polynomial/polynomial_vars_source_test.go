package polynomial

// Source: src/math/polynomial/polynomial.cpp:3186-3205,7359-7361;
// src/test/polynomial.cpp:837-879.

import (
	"errors"
	"sort"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestPolynomialVarsOriginalDormantAssertions(t *testing.T) {
	m := polynomialDiscriminantFixture(t)
	c := m.core
	parser := polynomialDiscriminantExpression{t: t, c: c}
	for i := range parser.variables {
		x, e := c.monomials.MkVar()
		serviceMust(t, e)
		parser.variables[i] = polynomialCaller(t, c, polynomialVarsVariable(t, c, x))
	}
	xs, e := NewPolynomialVariableDegrees(c.memory)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, xs.CloseChecked()) })
	// Literal original expressions with x0..x4 mapped to a,b,c,d,x solely for
	// the existing fixture parser. Original ENSURE size and sorted equality.
	rows := []struct {
		literal, expression string
		want                []uint32
	}{
		{"(x0 + 1)*((x0^2) + (x3^2))*(x2*x3)", "(a+1)*((a^2)+(d^2))*(c*d)", []uint32{0, 2, 3}},
		{"(x0 + x2)*((x0^2) + (x3^2))*(x2*x3)", "(a+c)*((a^2)+(d^2))*(c*d)", []uint32{0, 2, 3}},
		{"((x1 + x4 + 1)^5)", "((b+x+1)^5)", []uint32{1, 4}},
		{"((x1 + x4*x2 + 1)^4) + x0 + (x3^2)", "((b+x*c+1)^4)+a+(d^2)", []uint32{0, 1, 2, 3, 4}},
		{"(x3 + 1)^5", "(d+1)^5", []uint32{3}},
		{"x0*x1*x2", "a*b*c", []uint32{0, 1, 2}},
		{"x0*x1*x2 + 1", "a*b*c+1", []uint32{0, 1, 2}},
	}
	for _, row := range rows {
		p := parser.evaluate(row.expression)
		serviceMust(t, m.Vars(p, xs))
		n, e := xs.Len()
		serviceMust(t, e)
		if n != uint32(len(row.want)) {
			t.Fatalf("%s size %d", row.literal, n)
		}
		got := make([]uint32, n)
		for i := range got {
			got[i], e = xs.At(uint32(i))
			serviceMust(t, e)
		}
		if _, err := xs.At(n); !errors.Is(err, arena.ErrBounds) {
			t.Fatal("variable vector bounds", err)
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		for i := range got {
			if got[i] != row.want[i] {
				t.Fatalf("%s vars %v", row.literal, got)
			}
		}
	}
}
func polynomialVarsVariable(t *testing.T, c *MultivariatePolynomialCore, x uint32) Polynomial {
	t.Helper()
	v, e := c.MkVariable(x, 1)
	serviceMust(t, e)
	return v
}

func TestPolynomialVarsNativeColdPushAndNormalReset(t *testing.T) {
	m, p, _ := polynomialLexFixture(t)
	c := m.core
	xs, e := NewPolynomialVariableDegrees(c.memory)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, xs.CloseChecked()) })
	first, e := c.monomialCore(p, 0)
	serviceMust(t, e)
	x, e := c.monomials.varCore(first, 0)
	serviceMust(t, e)
	before, used := c.memory.Stats(), c.integer.Budget().Used()
	c.integer.Budget().FailAfter(0)
	returned := m.Vars(p, xs)
	c.integer.Budget().FailAfter(-1)
	serviceMust(t, returned)
	if c.memory.Stats() != before || c.integer.Budget().Used() != used {
		t.Fatal("cold native variable storage changed Runtime accounting")
	}
	n, e := xs.size()
	serviceMust(t, e)
	seen, e := c.substitution.found.at(x)
	serviceMust(t, e)
	if n != 2 || seen {
		t.Fatal("native push and normal reset", n, seen)
	}
	for i := uint32(0); i < n; i++ {
		collected, e := variableAt(xs, i)
		serviceMust(t, e)
		seen, e := c.substitution.found.at(collected)
		serviceMust(t, e)
		if seen {
			t.Fatal("normal suffix did not reset collected variable", collected)
		}
	}
	polynomialImpPhysicalAuditNamedRaw(t, m, []Polynomial{p}, nil, xs.header, nil)
	serviceMust(t, m.Vars(p, xs))
	n, e = xs.size()
	serviceMust(t, e)
	if n != 2 {
		t.Fatal("repeat collection", n)
	}
	seen, e = c.substitution.found.at(x)
	serviceMust(t, e)
	if seen {
		t.Fatal("normal suffix did not reset collected variable")
	}
}

func TestPolynomialVarsLiteralResetEmptyWarmAndBorrowGuards(t *testing.T) {
	m, p, _ := polynomialLexFixture(t)
	c := m.core
	xs, e := NewPolynomialVariableDegrees(c.memory)
	serviceMust(t, e)
	serviceMust(t, m.Vars(p, xs))
	n, e := xs.size()
	serviceMust(t, e)
	if n != 2 {
		t.Fatal(n)
	}
	// mkTerms places the maximal y^3 term first: this query preserves encounter
	// order and does not sort output or read coefficient values.
	first, e := variableAt(xs, 0)
	serviceMust(t, e)
	second, e := variableAt(xs, 1)
	serviceMust(t, e)
	if first != 1 || second != 0 {
		t.Fatal("first encounter order", first, second)
	}
	used := c.integer.Budget().Used()
	c.integer.Budget().FailAfter(0)
	serviceMust(t, m.Vars(p, xs))
	if used != c.integer.Budget().Used() {
		t.Fatal("warm variable storage changed")
	}
	serviceMust(t, m.Vars(c.zero, xs))
	n, e = xs.size()
	serviceMust(t, e)
	if n != 0 {
		t.Fatal("source empty output reset", n)
	}
	c.integer.Budget().FailAfter(-1)
	copied := *xs
	if e = m.Vars(p, &copied); e != ErrPolynomialOwner {
		t.Fatal("copied output", e)
	}
	ownerCopy := *m
	if e = ownerCopy.Vars(p, xs); e != ErrPolynomialOwner {
		t.Fatal("copied manager", e)
	}
	// Source reaches output reset/reserve before reading p.size. Invalid Go
	// input ownership is guarded at that reached borrow, not an early shortcut.
	serviceMust(t, variablePush(xs, 17))
	if e = m.Vars(Polynomial{}, xs); e == nil {
		t.Fatal("missing polynomial accepted")
	}
	n, e = xs.size()
	serviceMust(t, e)
	if n != 0 {
		t.Fatal("older output reset prefix erased", n)
	}
	serviceMust(t, xs.CloseChecked())
}

func TestPolynomialVarsNativeFailureRetainsVisitedPrefix(t *testing.T) {
	m, p, _ := polynomialLexFixture(t)
	c := m.core
	xs, e := NewPolynomialVariableDegrees(c.memory)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, xs.CloseChecked()) })
	first, e := c.monomialCore(p, 0)
	serviceMust(t, e)
	visited, e := c.monomials.varCore(first, 0)
	serviceMust(t, e)
	second, e := c.monomialCore(p, 1)
	serviceMust(t, e)
	saved, e := c.monomials.varCore(second, 0)
	serviceMust(t, e)
	location, e := c.monomials.location(second)
	serviceMust(t, e)
	// The actual field is restored before ordinary destruction. This fixture
	// exercises a reached bounds guard, not a completed source mathematical input.
	serviceMust(t, c.monomials.allocator.WriteUint32(location.raw, 20, ^uint32(0)))
	defer func() { serviceMust(t, c.monomials.allocator.WriteUint32(location.raw, 20, saved)) }()
	e = m.Vars(p, xs)
	wantError := error(arena.ErrBounds)
	if polynomialSourceDebug {
		wantError = ErrPolynomialAssertion
	}
	if !errors.Is(e, wantError) {
		t.Fatal("reached native flag bounds", e)
	}
	if len(xs.values) != 1 || xs.values[0] != visited {
		t.Fatal("published visited prefix", xs.values, visited)
	}
	for x, seen := range c.substitution.found.values {
		if seen != (uint32(x) == visited) {
			t.Fatal("source failure executed final reset", x, seen)
		}
	}
	polynomialImpPhysicalAuditNamedRaw(t, m, []Polynomial{p}, nil, xs.header, nil)
}
