package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"testing"
)

func TestProjectionIndexSignedBoundsAndRepeatedVariables(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	family, err := m.ArithmeticFamilyID()
	must(err)
	p := &projectionArithmetic{graph: graph, simplify: simplifier, family: family}
	defer p.close()
	zero, err := p.numeral("0")
	must(err)
	one, err := p.numeral("1")
	must(err)
	sort, err := m.GetSort(zero)
	must(err)
	name, err := m.Symbols.Intern("indexed-x")
	must(err)
	variable, err := p.pin(m.MkConst(name, sort))
	must(err)
	lower, err := p.app(3, variable, zero)
	must(err)
	upper, err := p.app(2, variable, one)
	must(err)
	index := newProjectionIndex(p)
	must(index.predicate(true, lower, 0))
	must(index.predicate(true, upper, 1))
	if index.indices[1][variable] != 0 || index.indices[0][variable] != 1 || len(index.positive) != 1 {
		t.Fatal("signed occurrences", index.indices, index.positive)
	}
	must(index.predicate(true, lower, 2))
	if index.indices[0][variable] != -1 || index.indices[1][variable] != -1 {
		t.Fatal("repeated occurrence remains eligible", index.indices)
	}
}

// AUDIT.md duality_02-F7: IndexLA's Times case takes arg(0) as the
// coefficient whatever the arity, so (<= (* 2 x y) 5) indexes x (as the
// non-numeral argument 1) with coefficient -2. (An n-ary - is built as a
// left-nested binary chain, so Sub's two-argument rule cannot be observed.)
func TestProjectionIndexTernaryMulUsesFirstTwoFactors(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	family, err := m.ArithmeticFamilyID()
	must(err)
	p := &projectionArithmetic{graph: graph, simplify: simplifier, family: family}
	defer p.close()
	two, err := p.numeral("2")
	must(err)
	five, err := p.numeral("5")
	must(err)
	sort, err := m.GetSort(two)
	must(err)
	constant := func(text string) ast.ASTHandle {
		name, err := m.Symbols.Intern(text)
		must(err)
		h, err := p.pin(m.MkConst(name, sort))
		must(err)
		return h
	}
	x, y, z := constant("ternary-x"), constant("ternary-y"), constant("ternary-z")
	product, err := p.app(9, two, x, y)
	must(err)
	le, err := p.app(2, product, five)
	must(err)
	index := newProjectionIndex(p)
	must(index.predicate(true, le, 0))
	if _, ok := index.indices[0][x]; !ok {
		t.Fatal("x not indexed through the ternary product", index.indices)
	}
	coefficient, err := p.normalized(index.coefficients[0][x])
	must(err)
	minusTwo, err := p.numeral("-2")
	must(err)
	if coefficient != minusTwo {
		t.Fatal("coefficient of x is not -2")
	}
	_ = z
}
