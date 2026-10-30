package subpaving

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fastMonomialFixture(t *testing.T, isInt []bool) (*numeral.FastFloatManager, *FastContext, *numeral.FastFloat, *FastNode) {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := numeral.NewScopedFastFloat(m)
	if err != nil {
		t.Fatal(err)
	}
	v, err := owner.Get()
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := NewFastContext(m, &control.Reslimit{Limit: 10000}, isInt, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	root, err := ctx.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := ctx.Close(); e != nil {
			t.Error(e)
		}
		if e := owner.Close(); e != nil {
			t.Error(e)
		}
		if e := m.CloseChecked(); e != nil {
			t.Error(e)
		}
		if budget.Used() != 0 {
			t.Error("retained monomial roots", budget.Used())
		}
	})
	return m, ctx, v, root
}

func fastMonomialBound(t *testing.T, m *numeral.FastFloatManager, c *FastContext, root *FastNode, v *numeral.FastFloat, variable uint32, value int32, lower bool) {
	t.Helper()
	if err := m.SetInt32(v, value); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PropagateCandidate(root, variable, v, lower, false); err != nil {
		t.Fatal(err)
	}
}

func fastMonomialCheckBound(t *testing.T, m *numeral.FastFloatManager, root *FastNode, v *numeral.FastFloat, variable uint32, value int32, lower bool) {
	t.Helper()
	b := root.upper[variable]
	if lower {
		b = root.lower[variable]
	}
	if b == nil {
		t.Fatalf("variable %d missing lower=%t", variable, lower)
	}
	if err := m.SetInt32(v, value); err != nil {
		t.Fatal(err)
	}
	equal, err := m.Equal(b.value, v)
	if err != nil || !equal {
		t.Fatalf("variable %d bound %d lower=%t: equal=%t err=%v", variable, value, lower, equal, err)
	}
}

func TestSubpavingFastMonomialMergedPowerForwardZeroAndQueue(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{true})
	x, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 1}, {Variable: 0, Degree: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if x != 1 || !c.isInt[x] || len(c.monomials[x].powers) != 1 || c.monomials[x].powers[0].Degree != 2 || len(c.watchers[0]) != 1 || c.watchers[0][0] != x {
		t.Fatal("merged source definition", x, c.monomials[x].powers, c.watchers)
	}
	fastMonomialBound(t, m, c, root, v, 0, 2, true)
	fastMonomialBound(t, m, c, root, v, 0, 3, false)
	if err := c.PropagateQueued(root); err != nil {
		t.Fatal(err)
	}
	fastMonomialCheckBound(t, m, root, v, x, 4, true)
	fastMonomialCheckBound(t, m, root, v, x, 9, false)
	if root.lower[x].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: x}) ||
		root.upper[x].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: x}) {
		t.Fatal("forward monomial justification")
	}
	if c.monomials[x].visited == 0 || c.numVisited == 0 {
		t.Fatal("watch queue did not visit monomial")
	}

	m2, c2, v2, root2 := fastMonomialFixture(t, []bool{false})
	y, err := c2.DefineMonomial([]FastPower{{Variable: 0, Degree: 2}})
	if err != nil {
		t.Fatal(err)
	}
	fastMonomialBound(t, m2, c2, root2, v2, 0, 0, false)
	if err := c2.PropagateMonomial(y, root2); err != nil {
		t.Fatal(err)
	}
	zero, err := root2.IsZero(y)
	if err != nil || !zero {
		t.Fatal("even upper-zero factor did not force zero", zero, err)
	}
	if root2.lower[y].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: y}) ||
		root2.upper[y].reason != (fastBoundReason{kind: fastReasonVariableDefinition, variable: y}) {
		t.Fatal("zero monomial justification")
	}
}

func TestSubpavingFastMonomialDegreeMergeUsesUnsignedArithmetic(t *testing.T) {
	_, c, _, _ := fastMonomialFixture(t, []bool{false})
	max := ^uint32(0)
	x, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: max}, {Variable: 0, Degree: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.monomials[x].powers) != 1 || c.monomials[x].powers[0].Degree != 1 {
		t.Fatal("source unsigned degree merge", c.monomials[x].powers)
	}
	wrapped, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: max}, {Variable: 0, Degree: 1}})
	if err != nil || c.monomials[wrapped].powers[0].Degree != 0 {
		t.Fatal("source unsigned degree merge preserves zero wrap", wrapped, err)
	}
	zero, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 0}})
	if err != nil || c.monomials[zero].powers[0].Degree != 0 {
		t.Fatal("source mk_monomial accepts a zero degree", zero, err)
	}
}

func TestSubpavingFastQueuedMonomialCatchesMPFFOverflow(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false})
	y, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 2}})
	if err != nil {
		t.Fatal(err)
	}
	z, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetMax(v); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(root, 0, v, true, false); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(root, 0, v, false, false); err != nil {
		t.Fatal(err)
	}
	if len(c.queue) != 2 {
		t.Fatal("source watched bound was not queued", len(c.queue))
	}
	if err := c.PropagateQueued(root); err != nil || !c.arithmeticFailed || root.lower[y] != nil || root.upper[y] != nil || len(c.queue) != 0 {
		t.Fatalf("watched MPFF overflow catch: err=%v, failed=%t, derived=(%p,%p), queue=%d", err, c.arithmeticFailed, root.lower[y], root.upper[y], len(c.queue))
	}
	if root.lower[z] == nil || root.upper[z] == nil {
		t.Fatal("later watched definition did not run after MPFF overflow")
	}
	for _, bound := range []*FastBound{root.lower[z], root.upper[z]} {
		equal, err := m.Equal(bound.value, v)
		if err != nil || !equal {
			t.Fatalf("later watched definition lost the exact maximum bound: equal=%t err=%v", equal, err)
		}
	}
}

func TestSubpavingFastQueuedSelfDefinitionPropagatesMPFFOverflow(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false, false})
	y, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 1}, {Variable: 1, Degree: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetPlusEpsilon(v); err != nil {
		t.Fatal(err)
	}
	for _, lower := range []bool{true, false} {
		if err := c.PropagateBound(root, 0, v, lower, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SetMax(v); err != nil {
		t.Fatal(err)
	}
	for _, lower := range []bool{true, false} {
		if err := c.PropagateBound(root, y, v, lower, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.processQueuedBound(root, root.lower[y]); !errors.Is(err, numeral.ErrFastFloatDomain) || c.arithmeticFailed {
		t.Fatalf("source self-definition exception must escape watcher catch: %v, failed=%t", err, c.arithmeticFailed)
	}
}

func TestSubpavingFastMonomialReverseAndSourceScratch(t *testing.T) {
	t.Run("cubic", func(t *testing.T) {
		m, c, v, root := fastMonomialFixture(t, []bool{false})
		x, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 3}})
		if err != nil {
			t.Fatal(err)
		}
		fastMonomialBound(t, m, c, root, v, x, 8, true)
		fastMonomialBound(t, m, c, root, v, x, 27, false)
		if err := c.PropagateMonomial(x, root); err != nil {
			t.Fatal(err)
		}
		if root.lower[0] == nil || root.upper[0] == nil || root.inconsistent {
			t.Fatal("missing cubic reverse bounds")
		}
		if err := m.SetInt32(v, 2); err != nil {
			t.Fatal(err)
		}
		tooHigh, err := m.Greater(root.lower[0].value, v)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.SetInt32(v, 3); err != nil {
			t.Fatal(err)
		}
		tooLow, err := m.Less(root.upper[0].value, v)
		if err != nil || tooHigh || tooLow {
			t.Fatal("cubic reverse enclosure", tooHigh, tooLow, err)
		}
	})
	t.Run("product", func(t *testing.T) {
		m, c, v, root := fastMonomialFixture(t, []bool{false, true})
		x, err := c.DefineMonomial([]FastPower{{Variable: 1, Degree: 1}, {Variable: 0, Degree: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if !c.isInt[x] || c.monomials[x].powers[0].Variable != 0 {
			t.Fatal("source integer-any/sorted definition")
		}
		fastMonomialBound(t, m, c, root, v, 0, 2, true)
		fastMonomialBound(t, m, c, root, v, 0, 3, false)
		fastMonomialBound(t, m, c, root, v, x, 8, true)
		fastMonomialBound(t, m, c, root, v, x, 15, false)
		if err := c.PropagateMonomial(x, root); err != nil {
			t.Fatal(err)
		}
		fastMonomialCheckBound(t, m, root, v, 1, 3, true)
		fastMonomialCheckBound(t, m, root, v, 1, 7, false)
	})
	t.Run("three-factor-source-scratch", func(t *testing.T) {
		m, c, v, root := fastMonomialFixture(t, []bool{false, false, false})
		x, err := c.DefineMonomial([]FastPower{{0, 1}, {1, 1}, {2, 1}})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			variable uint32
			value    int32
		}{{0, 2}, {1, 3}, {x, 30}} {
			fastMonomialBound(t, m, c, root, v, item.variable, item.value, true)
			fastMonomialBound(t, m, c, root, v, item.variable, item.value, false)
		}
		if err := c.PropagateMonomial(x, root); err != nil {
			t.Fatal(err)
		}
		// The source never clears its first flag, so d is overwritten by the
		// last factor other than the target (x1=3): 30/3 = 10.
		fastMonomialCheckBound(t, m, root, v, 2, 10, true)
		fastMonomialCheckBound(t, m, root, v, 2, 10, false)
	})
}

func TestSubpavingFastMonomialIntervalDisplaySmoke(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false})
	x, err := c.DefineMonomial([]FastPower{{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	fastMonomialBound(t, m, c, root, v, 0, 2, true)
	fastMonomialBound(t, m, c, root, v, 0, 3, false)
	if err := c.PropagateMonomial(x, root); err != nil {
		t.Fatal(err)
	}
	view, err := NewFastInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	if err := view.SetConstant(root, x); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := view.Display(&out); err != nil || out.String() != "[2, 3]" {
		t.Fatal("monomial view", out.String(), err)
	}
}
