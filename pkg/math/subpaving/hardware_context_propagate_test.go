package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestHardwareContextRootDefinitionOrderAndQueuedWatchers(t *testing.T) {
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	limit := &control.Reslimit{Limit: 3}
	c, err := NewHardwareContext(m, limit, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	x, err := c.NewVariable(false)
	if err != nil {
		t.Fatal(err)
	}
	var one, two, three, half numeral.HardwareFloat
	for _, entry := range []struct {
		out   *numeral.HardwareFloat
		value float64
	}{{&one, 1}, {&two, 2}, {&three, 3}, {&half, 1.5}} {
		if err := m.SetFloat64(entry.out, entry.value); err != nil {
			t.Fatal(err)
		}
	}
	y, err := c.DefinePolynomial(&one, []HardwarePolynomialTerm{{Variable: x, Coefficient: &two}})
	if err != nil {
		t.Fatal(err)
	}
	z, err := c.DefinePolynomial(&one, []HardwarePolynomialTerm{{Variable: y, Coefficient: &three}})
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []struct {
		value *numeral.HardwareFloat
		lower bool
	}{{&one, true}, {&two, false}} {
		a, err := c.NewInequality(x, unit.value, unit.lower, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddUnitClause(a, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.InitializeRoot(); !errors.Is(err, ErrHardwareContextCanceled) {
		t.Fatal("queue checkpoint after root units and definitions", err)
	}
	n := c.root
	if n == nil || c.initStage != 2 || c.unitHead != 2 || c.initDefHead != 3 || c.queueHead != 1 ||
		len(c.queue) != 6 || c.numVisited != 2 ||
		m.ToFloat64(&n.lower[y].value) != 2 || m.ToFloat64(&n.upper[y].value) != 4 ||
		m.ToFloat64(&n.lower[z].value) != 6 || m.ToFloat64(&n.upper[z].value) != 12 {
		t.Fatal("source root order or completed prefix before cancellation")
	}
	limit.Limit = 100
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	if c.initStage != 2 || c.unitHead != 2 || len(c.queue) == 0 || c.queueHead != 1 ||
		m.ToFloat64(&n.lower[z].value) != 6 || m.ToFloat64(&n.upper[z].value) != 12 {
		t.Fatal("later call drained the source queue after cancellation")
	}
	if err := c.PropagateBound(n, x, &half, true, false); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateQueued(n); err != nil {
		t.Fatal(err)
	}
	if m.ToFloat64(&n.lower[y].value) != 3 || m.ToFloat64(&n.lower[z].value) != 9 ||
		m.ToFloat64(&n.upper[y].value) != 4 || m.ToFloat64(&n.upper[z].value) != 12 ||
		len(c.queue) != 0 || c.queueHead != 0 || c.polynomials[y].visited == 0 || c.polynomials[z].visited == 0 {
		t.Fatal("queued x bound must visit chained polynomial watchers")
	}
	before := c.numVisited
	if err := c.InitializeRoot(); err != nil || c.numVisited != before {
		t.Fatal("completed root initialization must be idempotent", err)
	}
}
