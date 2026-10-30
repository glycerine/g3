package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastPolynomialForwardReverseAndOwnedCoefficients(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owners := make([]*numeral.ScopedFastFloat, 5)
	for i := range owners {
		owners[i], err = numeral.NewScopedFastFloat(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	values := make([]*numeral.FastFloat, len(owners))
	for i, owner := range owners {
		values[i], err = owner.Get()
		if err != nil {
			t.Fatal(err)
		}
	}
	var contexts []*FastContext
	defer func() {
		for i := len(contexts) - 1; i >= 0; i-- {
			if e := contexts[i].Close(); e != nil {
				t.Error(e)
			}
		}
		for i := len(owners) - 1; i >= 0; i-- {
			if e := owners[i].Close(); e != nil {
				t.Error(e)
			}
		}
		if e := m.CloseChecked(); e != nil {
			t.Error(e)
		}
		if budget.Used() != 0 {
			t.Error("retained fast-float roots", budget.Used())
		}
	}()
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	constant, coefficient, value, expectedLo, expectedHi := values[0], values[1], values[2], values[3], values[4]
	must(m.SetInt32(constant, 0))
	must(m.SetInt32(coefficient, 1))
	newContext := func() *FastContext {
		t.Helper()
		c, e := NewFastContext(m, &control.Reslimit{Limit: 1000}, []bool{false, false}, 0, 2, 8192)
		must(e)
		contexts = append(contexts, c)
		return c
	}
	define := func(c *FastContext) uint32 {
		t.Helper()
		x, e := c.DefinePolynomial(constant, []FastPolynomialTerm{{Variable: 0, Coefficient: coefficient}, {Variable: 1, Coefficient: coefficient}})
		must(e)
		if x != 2 || len(c.watchers[0]) != 1 || len(c.watchers[1]) != 1 {
			t.Fatal("polynomial definition/watchers")
		}
		return x
	}
	bound := func(c *FastContext, n *FastNode, variable uint32, amount int32, lower bool) {
		t.Helper()
		must(m.SetInt32(value, amount))
		must(c.PropagateBound(n, variable, value, lower, false))
	}
	check := func(n *FastNode, variable uint32, lo, hi int32) {
		t.Helper()
		lower, e := n.bound(variable, true)
		must(e)
		upper, e := n.bound(variable, false)
		must(e)
		if lower == nil || upper == nil || lower.open || upper.open {
			t.Fatal("missing or open polynomial bounds")
		}
		must(m.SetInt32(expectedLo, lo))
		must(m.SetInt32(expectedHi, hi))
		le, e := m.Equal(lower.value, expectedLo)
		must(e)
		ue, e := m.Equal(upper.value, expectedHi)
		must(e)
		if !le || !ue {
			t.Fatalf("polynomial bounds x%d: lower=%t upper=%t", variable, le, ue)
		}
	}
	forward := newContext()
	x := define(forward)
	root, err := forward.Root()
	must(err)
	bound(forward, root, 0, 1, true)
	bound(forward, root, 0, 2, false)
	bound(forward, root, 1, 3, true)
	bound(forward, root, 1, 4, false)
	must(forward.PropagatePolynomial(x, root))
	check(root, x, 4, 6)
	if root.lower[x].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: x}) ||
		root.upper[x].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: x}) {
		t.Fatal("forward polynomial justification")
	}
	if forward.polynomials[x].visited != forward.timestamp-2 {
		t.Fatal("polynomial visitation timestamp")
	}
	reverse := newContext()
	x = define(reverse)
	root, err = reverse.Root()
	must(err)
	bound(reverse, root, 0, 1, true)
	bound(reverse, root, 0, 2, false)
	bound(reverse, root, x, 4, true)
	bound(reverse, root, x, 6, false)
	must(reverse.PropagatePolynomial(x, root))
	check(root, 1, 2, 5)
	if root.lower[1].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: x}) ||
		root.upper[1].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: x}) {
		t.Fatal("reverse polynomial justification")
	}
	must(m.SetInt32(coefficient, 9))
	if reverse.polynomials[x].terms[0].Coefficient == coefficient {
		t.Fatal("polynomial retained borrowed coefficient")
	}
	check(root, 1, 2, 5)
}

func TestSubpavingFastPolynomialDuplicateVariablesKeepSourceSlots(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owners := make([]*numeral.ScopedFastFloat, 5)
	for i := range owners {
		owners[i], err = numeral.NewScopedFastFloat(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for i := len(owners) - 1; i >= 0; i-- {
			if e := owners[i].Close(); e != nil {
				t.Error(e)
			}
		}
		if e := m.CloseChecked(); e != nil {
			t.Error(e)
		}
		if budget.Used() != 0 {
			t.Error("polynomial owners remain", budget.Used())
		}
	}()
	values := make([]*numeral.FastFloat, len(owners))
	for i := range owners {
		values[i], err = owners[i].Get()
		if err != nil {
			t.Fatal(err)
		}
	}
	constant, coefficient2, boundValue, coefficient3, coefficient4 := values[0], values[1], values[2], values[3], values[4]
	for x, n := range map[*numeral.FastFloat]int32{constant: 0, coefficient2: 2, coefficient3: 3, coefficient4: 4} {
		if err := m.SetInt32(x, n); err != nil {
			t.Fatal(err)
		}
	}
	c, err := NewFastContext(m, &control.Reslimit{Limit: 1000}, []bool{false, false}, 0, 10, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	x, err := c.DefinePolynomial(constant, []FastPolynomialTerm{
		{Variable: 1, Coefficient: coefficient2},
		{Variable: 0, Coefficient: coefficient3},
		{Variable: 1, Coefficient: coefficient4},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := c.polynomials[x]
	if len(p.terms) != 3 || p.terms[0].Variable != 0 || p.terms[1].Variable != 1 || p.terms[2].Variable != 1 {
		t.Fatalf("source-sorted polynomial slots = %#v", p.terms)
	}
	for i, want := range []string{"3", "4", "0"} {
		got, err := m.String(p.terms[i].Coefficient)
		if err != nil || got != want {
			t.Fatalf("coefficient slot %d = %q, %v; want %q", i, got, err, want)
		}
	}
	if len(c.watchers[0]) != 1 || len(c.watchers[1]) != 2 {
		t.Fatalf("source watcher multiplicity = %v", c.watchers)
	}
	root, err := c.Root()
	if err != nil {
		t.Fatal(err)
	}
	setBound := func(variable uint32, amount int32, lower bool) {
		t.Helper()
		if err := m.SetInt32(boundValue, amount); err != nil {
			t.Fatal(err)
		}
		if err := c.PropagateBound(root, variable, boundValue, lower, false); err != nil {
			t.Fatal(err)
		}
	}
	setBound(0, 1, true)
	setBound(0, 2, false)
	setBound(1, 5, true)
	setBound(1, 6, false)
	if err := c.PropagatePolynomial(x, root); err != nil {
		t.Fatal(err)
	}
	// The source leaves the duplicate zero coefficient as a real term. Its
	// scalar multiply resets that scratch interval to unbounded, so the later
	// addition removes both candidate bounds.
	if root.lower[x] != nil || root.upper[x] != nil {
		t.Fatal("zero duplicate coefficient should reset the source propagation scratch")
	}
}
