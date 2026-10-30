package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestHardwareMonomialDefinitionForwardAndZero(t *testing.T) {
	newContext := func(t *testing.T) (*HardwareContext, *numeral.HardwareFloatNumeralManager, uint32) {
		t.Helper()
		m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
		if err != nil {
			t.Fatal(err)
		}
		c, err := NewHardwareContext(m, &control.Reslimit{Limit: 1000}, 0, 2, 8192)
		if err != nil {
			t.Fatal(err)
		}
		x, err := c.NewVariable(true)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		})
		return c, m, x
	}
	t.Run("zero-degree source definition", func(t *testing.T) {
		c, _, x := newContext(t)
		out, err := c.DefineMonomial([]HardwarePower{{Variable: x, Degree: 0}})
		if err != nil || c.monomials[out].powers[0].Degree != 0 {
			t.Fatal("source mk_monomial accepts a zero degree", out, err)
		}
	})
	t.Run("merged-power-and-queue", func(t *testing.T) {
		c, m, x := newContext(t)
		out, err := c.DefineMonomial([]HardwarePower{{x, 1}, {x, 1}})
		if err != nil {
			t.Fatal(err)
		}
		if !c.isInt[out] || len(c.monomials[out].powers) != 1 || c.monomials[out].powers[0].Degree != 2 ||
			len(c.watchOrder[x]) != 1 || c.watchOrder[x][0].definition != out {
			t.Fatal("merged definition and unique watcher")
		}
		n, err := c.CreateRootNode()
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range []struct {
			value int32
			lower bool
		}{{2, true}, {3, false}} {
			var value numeral.HardwareFloat
			if err := m.SetInt32(&value, b.value); err != nil {
				t.Fatal(err)
			}
			if err := c.PropagateBound(n, x, &value, b.lower, false); err != nil {
				t.Fatal(err)
			}
		}
		if err := c.PropagateQueued(n); err != nil {
			t.Fatal(err)
		}
		if n.lower[out] == nil || n.upper[out] == nil || m.ToFloat64(&n.lower[out].value) != 4 ||
			m.ToFloat64(&n.upper[out].value) != 9 || n.lower[out].reasonVariable != out ||
			c.monomials[out].visited == 0 || c.numVisited == 0 {
			t.Fatal("queued forward monomial bounds")
		}
	})
	t.Run("even-upper-zero", func(t *testing.T) {
		c, m, x := newContext(t)
		out, err := c.DefineMonomial([]HardwarePower{{x, 2}})
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.CreateRootNode()
		if err != nil {
			t.Fatal(err)
		}
		var zero numeral.HardwareFloat
		if err := m.SetInt32(&zero, 0); err != nil {
			t.Fatal(err)
		}
		if err := c.PropagateBound(n, x, &zero, false, false); err != nil {
			t.Fatal(err)
		}
		if err := c.PropagateMonomial(out, n); err != nil {
			t.Fatal(err)
		}
		isZero, err := n.IsZero(out)
		if err != nil || !isZero || n.lower[out].reason != hardwareReasonVariableDefinition ||
			n.upper[out].reasonVariable != out {
			t.Fatal("even power upper zero forces output zero", err)
		}
	})
}

func TestHardwareMonomialReverseCubicAndSourceProduct(t *testing.T) {
	newContext := func(t *testing.T, count int) (*HardwareContext, *numeral.HardwareFloatNumeralManager, []uint32) {
		t.Helper()
		m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
		if err != nil {
			t.Fatal(err)
		}
		c, err := NewHardwareContext(m, &control.Reslimit{Limit: 10000}, 0, 2, 8192)
		if err != nil {
			t.Fatal(err)
		}
		var xs []uint32
		for range count {
			x, err := c.NewVariable(false)
			if err != nil {
				t.Fatal(err)
			}
			xs = append(xs, x)
		}
		t.Cleanup(func() {
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		})
		return c, m, xs
	}
	bound := func(t *testing.T, c *HardwareContext, m *numeral.HardwareFloatNumeralManager, n *HardwareNode, x uint32, value int32, lower bool) {
		t.Helper()
		var v numeral.HardwareFloat
		if err := m.SetInt32(&v, value); err != nil {
			t.Fatal(err)
		}
		if err := c.PropagateBound(n, x, &v, lower, false); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("cubic-inverse", func(t *testing.T) {
		c, m, xs := newContext(t, 1)
		out, err := c.DefineMonomial([]HardwarePower{{xs[0], 3}})
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.CreateRootNode()
		if err != nil {
			t.Fatal(err)
		}
		bound(t, c, m, n, out, 8, true)
		bound(t, c, m, n, out, 27, false)
		if err := c.PropagateMonomial(out, n); err != nil {
			t.Fatal(err)
		}
		if n.lower[xs[0]] == nil || n.upper[xs[0]] == nil ||
			m.ToFloat64(&n.lower[xs[0]].value) > 2 || m.ToFloat64(&n.upper[xs[0]].value) < 3 ||
			n.lower[xs[0]].reasonVariable != out {
			t.Fatal("cubic inverse enclosure")
		}
	})
	t.Run("three-factor-scratch", func(t *testing.T) {
		c, m, xs := newContext(t, 3)
		out, err := c.DefineMonomial([]HardwarePower{{xs[0], 1}, {xs[1], 1}, {xs[2], 1}})
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.CreateRootNode()
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			x uint32
			v int32
		}{{xs[0], 2}, {xs[1], 3}, {out, 30}} {
			bound(t, c, m, n, item.x, item.v, true)
			bound(t, c, m, n, item.x, item.v, false)
		}
		if err := c.PropagateMonomial(out, n); err != nil {
			t.Fatal(err)
		}
		if n.lower[xs[2]] == nil || n.upper[xs[2]] == nil ||
			m.ToFloat64(&n.lower[xs[2]].value) != 15 ||
			m.ToFloat64(&n.upper[xs[2]].value) != 15 {
			t.Fatal("source denominator stays at first factor")
		}
	})
}
