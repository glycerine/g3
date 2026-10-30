package subpaving

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFixedMonomialMergedPowerForwardAndZero(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 10000}, []bool{true}, 0, 2, 8192)
	must(err)
	x, err := ctx.DefineMonomial([]FixedPower{{Variable: 0, Degree: 1}, {Variable: 0, Degree: 1}})
	must(err)
	if x != 1 || !ctx.isInt[x] || len(ctx.monomials[x].powers) != 1 || ctx.monomials[x].powers[0].Degree != 2 ||
		len(ctx.watchers[0]) != 1 || ctx.watchers[0][0] != x {
		t.Fatalf("merged definition: x=%d powers=%v integer=%v watchers=%v", x, ctx.monomials[x].powers, ctx.isInt[x], ctx.watchers[0])
	}
	root, err := ctx.Root()
	must(err)
	must(m.SetInt64(v, 2))
	must(ctx.PropagateBound(root, 0, v, true, false))
	must(m.SetInt64(v, 3))
	must(ctx.PropagateBound(root, 0, v, false, false))
	must(ctx.PropagateMonomial(x, root))
	view, err := NewFixedInterval(m)
	must(err)
	must(view.SetConstant(root, x))
	var out bytes.Buffer
	must(view.Display(&out))
	if got := out.String(); got != "[4, 9]" {
		t.Fatalf("squared bounds = %s, want [4, 9]", got)
	}
	if root.lower[x].reason != (fixedBoundReason{kind: fixedReasonVariableDefinition, variable: x}) ||
		root.upper[x].reason != (fixedBoundReason{kind: fixedReasonVariableDefinition, variable: x}) {
		t.Fatal("forward monomial justification")
	}
	must(view.Close())
	must(ctx.Close())
	must(v.Close())
	must(m.Close())

	m, err = numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err = m.New()
	must(err)
	ctx, err = NewFixedContext(m, &control.Reslimit{Limit: 10000}, []bool{false}, 0, 2, 8192)
	must(err)
	x, err = ctx.DefineMonomial([]FixedPower{{Variable: 0, Degree: 2}})
	must(err)
	root, err = ctx.Root()
	must(err)
	must(m.SetInt64(v, 0))
	must(ctx.PropagateBound(root, 0, v, false, false))
	must(ctx.PropagateMonomial(x, root))
	zero, err := root.IsZero(x)
	must(err)
	if !zero || len(ctx.queue) != 3 {
		t.Fatalf("even upper-zero factor did not force output zero: zero=%v queue=%d", zero, len(ctx.queue))
	}
	if root.lower[x].reason != (fixedBoundReason{kind: fixedReasonVariableDefinition, variable: x}) ||
		root.upper[x].reason != (fixedBoundReason{kind: fixedReasonVariableDefinition, variable: x}) {
		t.Fatal("zero monomial justification")
	}
	must(ctx.Close())
	must(v.Close())
	must(m.Close())
}

func TestSubpavingFixedMonomialAcceptsSourceZeroDegreeAndWrap(t *testing.T) {
	m, err := numeral.NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFixedContext(m, &control.Reslimit{Limit: 10000}, []bool{false}, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	zero, err := c.DefineMonomial([]FixedPower{{Variable: 0, Degree: 0}})
	if err != nil || c.monomials[zero].powers[0].Degree != 0 {
		t.Fatal("source mk_monomial accepts a zero degree", zero, err)
	}
	wrapped, err := c.DefineMonomial([]FixedPower{{Variable: 0, Degree: ^uint32(0)}, {Variable: 0, Degree: 1}})
	if err != nil || c.monomials[wrapped].powers[0].Degree != 0 {
		t.Fatal("source unsigned degree merge preserves zero wrap", wrapped, err)
	}
}

func TestSubpavingFixedMonomialOddUnboundedDownward(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 10000}, []bool{false}, 0, 2, 8192)
	must(err)
	x, err := ctx.DefineMonomial([]FixedPower{{Variable: 0, Degree: 3}})
	must(err)
	root, err := ctx.Root()
	must(err)
	must(m.SetInt64(v, 8))
	must(ctx.PropagateBound(root, x, v, true, false))
	must(m.SetInt64(v, 27))
	must(ctx.PropagateBound(root, x, v, false, false))
	must(ctx.PropagateMonomial(x, root))
	if root.lower[0] == nil || root.upper[0] == nil {
		t.Fatal("bounded cubic output did not propagate to unbounded input")
	}
	lo, err := m.Rational(root.lower[0].value)
	must(err)
	hi, err := m.Rational(root.upper[0].value)
	must(err)
	if lo.Cmp(big.NewRat(2, 1)) > 0 || hi.Cmp(big.NewRat(3, 1)) < 0 || root.inconsistent {
		t.Fatalf("cubic reverse interval [%s,%s], conflict=%v", lo, hi, root.inconsistent)
	}
	must(ctx.Close())
	must(v.Close())
	must(m.Close())
}

func TestSubpavingFixedMonomialProductDownward(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 10000}, []bool{false, true}, 0, 2, 8192)
	must(err)
	x, err := ctx.DefineMonomial([]FixedPower{{Variable: 1, Degree: 1}, {Variable: 0, Degree: 1}})
	must(err)
	if !ctx.isInt[x] || ctx.monomials[x].powers[0].Variable != 0 {
		t.Fatalf("source integer-any/sorted metadata: integer=%v powers=%v", ctx.isInt[x], ctx.monomials[x].powers)
	}
	root, err := ctx.Root()
	must(err)
	bound := func(variable uint32, n int64, lower bool) {
		t.Helper()
		must(m.SetInt64(v, n))
		must(ctx.PropagateBound(root, variable, v, lower, false))
	}
	bound(0, 2, true)
	bound(0, 3, false)
	bound(x, 8, true)
	bound(x, 15, false)
	must(ctx.PropagateMonomial(x, root))
	if root.lower[1] == nil || root.upper[1] == nil {
		t.Fatal("second factor did not receive reverse bounds")
	}
	lo, err := m.Rational(root.lower[1].value)
	must(err)
	hi, err := m.Rational(root.upper[1].value)
	must(err)
	if lo.Cmp(big.NewRat(3, 1)) != 0 || hi.Cmp(big.NewRat(7, 1)) != 0 || root.inconsistent {
		t.Fatalf("integer-normalized product reverse interval [%s,%s], conflict=%v", lo, hi, root.inconsistent)
	}
	must(ctx.Close())
	must(v.Close())
	must(m.Close())
}

func TestSubpavingFixedMonomialSourceThreeFactorDownwardScratch(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 10000}, []bool{false, false, false}, 0, 2, 8192)
	must(err)
	x, err := ctx.DefineMonomial([]FixedPower{{0, 1}, {1, 1}, {2, 1}})
	must(err)
	root, err := ctx.Root()
	must(err)
	bound := func(variable uint32, n int64, lower bool) {
		t.Helper()
		must(m.SetInt64(v, n))
		must(ctx.PropagateBound(root, variable, v, lower, false))
	}
	for _, item := range []struct {
		variable uint32
		value    int64
	}{{0, 2}, {1, 3}, {x, 30}} {
		bound(item.variable, item.value, true)
		bound(item.variable, item.value, false)
	}
	must(ctx.PropagateMonomial(x, root))
	if root.lower[2] == nil || root.upper[2] == nil {
		t.Fatal("third factor stayed unbounded")
	}
	lo, err := m.Rational(root.lower[2].value)
	must(err)
	hi, err := m.Rational(root.upper[2].value)
	must(err)
	// The retained source leaves d at the first other factor (2), rather
	// than the computed product (6), when it divides the output interval.
	if lo.Cmp(big.NewRat(15, 1)) != 0 || hi.Cmp(big.NewRat(15, 1)) != 0 {
		t.Fatalf("source downward scratch bounds [%s,%s], want [15,15]", lo, hi)
	}
	must(ctx.Close())
	must(v.Close())
	must(m.Close())
}
