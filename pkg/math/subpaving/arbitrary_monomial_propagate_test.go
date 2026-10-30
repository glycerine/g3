package subpaving

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestArbitraryMonomialForwardInverseAndArithmetic(t *testing.T) {
	bound := func(t *testing.T, f *arbitraryClauseFixture, n *ArbitraryNode, variable uint32, value int32, lower bool) {
		t.Helper()
		var v numeral.ArbitraryFloat
		if err := f.numbers.SetInt32(&v, value); err != nil {
			t.Fatal(err)
		}
		if err := f.context.PublishBound(n, variable, &v, lower, false, true); err != nil {
			t.Fatal(err)
		}
		if err := f.numbers.Del(&v); err != nil {
			t.Fatal(err)
		}
	}
	value := func(t *testing.T, f *arbitraryClauseFixture, b *ArbitraryBound) *big.Rat {
		t.Helper()
		if b == nil {
			t.Fatal("missing monomial bound")
		}
		core, err := f.numbers.BorrowCore()
		if err != nil {
			t.Fatal(err)
		}
		v, err := core.ToDouble(&b.value)
		if err != nil {
			t.Fatal(err)
		}
		return new(big.Rat).SetFloat64(v)
	}
	t.Run("zero-degree source definition", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		out, err := f.context.DefineMonomial([]ArbitraryPower{{Variable: f.x, Degree: 0}})
		if err != nil || f.context.monomials[out].powers[0].Degree != 0 {
			t.Fatal("source mk_monomial accepts a zero degree", out, err)
		}
	})
	t.Run("merged even power and queue", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		c := f.context
		out, err := c.DefineMonomial([]ArbitraryPower{{f.x, 1}, {f.x, 1}})
		if err != nil {
			t.Fatal(err)
		}
		if len(c.monomials[out].powers) != 1 || c.monomials[out].powers[0].Degree != 2 || len(c.watchOrder[f.x]) != 1 {
			t.Fatal("merged source power or unique watcher")
		}
		n, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		bound(t, f, n, f.x, 2, true)
		bound(t, f, n, f.x, 3, false)
		if err := c.PropagateQueued(n); err != nil {
			t.Fatal(err)
		}
		if value(t, f, n.lower[out]).Cmp(big.NewRat(4, 1)) != 0 || value(t, f, n.upper[out]).Cmp(big.NewRat(9, 1)) != 0 ||
			!n.lower[out].definitionReason || n.upper[out].reasonVariable != out || c.monomials[out].visited == 0 || len(c.queue) != 0 {
			t.Fatal("queued even-power forward bounds or reason")
		}
	})
	t.Run("cubic inverse", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		c := f.context
		out, err := c.DefineMonomial([]ArbitraryPower{{f.x, 3}})
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		bound(t, f, n, out, 8, true)
		bound(t, f, n, out, 27, false)
		if err := c.PropagateMonomial(out, n); err != nil {
			t.Fatal(err)
		}
		if value(t, f, n.lower[f.x]).Cmp(big.NewRat(2, 1)) > 0 || value(t, f, n.upper[f.x]).Cmp(big.NewRat(3, 1)) < 0 ||
			!n.lower[f.x].definitionReason || n.upper[f.x].reasonVariable != out {
			t.Fatal("cubic inverse must enclose exact roots and retain definition reason")
		}
	})
	t.Run("three-factor source denominator", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		c := f.context
		z, err := c.NewVariable(false)
		if err != nil {
			t.Fatal(err)
		}
		out, err := c.DefineMonomial([]ArbitraryPower{{f.x, 1}, {f.y, 1}, {z, 1}})
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			variable uint32
			value    int32
		}{{f.x, 2}, {f.y, 3}, {out, 30}} {
			bound(t, f, n, item.variable, item.value, true)
			bound(t, f, n, item.variable, item.value, false)
		}
		if err := c.PropagateMonomial(out, n); err != nil {
			t.Fatal(err)
		}
		if value(t, f, n.lower[z]).Cmp(big.NewRat(15, 1)) != 0 || value(t, f, n.upper[z]).Cmp(big.NewRat(15, 1)) != 0 {
			t.Fatal("source denominator retains first factor rather than full product")
		}
	})
	t.Run("nonregular watched arithmetic is recorded", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		c := f.context
		out, err := c.DefineMonomial([]ArbitraryPower{{f.x, 2}})
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		var huge numeral.ArbitraryFloat
		if err := f.numbers.SetDouble(&huge, 1e308); err != nil {
			t.Fatal(err)
		}
		if err := c.PublishBound(n, f.x, &huge, true, false, true); err != nil {
			t.Fatal(err)
		}
		if err := c.PublishBound(n, f.x, &huge, false, false, true); err != nil {
			t.Fatal(err)
		}
		if err := f.numbers.Del(&huge); err != nil {
			t.Fatal(err)
		}
		if err := c.PropagateQueued(n); err != nil {
			t.Fatal(err)
		}
		if !c.arithmeticFailed || n.lower[out] != nil || len(c.queue) != 0 {
			t.Fatal("watched f2n arithmetic exception must mark failure and drain queue")
		}
	})
}
