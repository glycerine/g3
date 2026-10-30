package arith

// Source: src/util/trace.h TRACE stream badbit behavior at
// src/tactic/arith/nla2bv_tactic.cpp:85-88,111.

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type nla2BVRefusingTrace struct{ writes int }

func (w *nla2BVRefusingTrace) Write([]byte) (int, error) {
	w.writes++
	return 0, io.ErrClosedPipe
}

func TestNLA2BVTraceRefusalDoesNotChangeCountingOrExecution(t *testing.T) {
	r := nla2BVFixture(t, false)
	w := &nla2BVRefusingTrace{}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("nla2bv")
	if !tags.ShouldTrace("nla2bv") {
		t.Skip("source TRACE macro erased")
	}
	equationMust(t, r.BindTrace(tags))
	m := r.manager
	x := fixDLConst(t, m, true, "trace-refusal-x")
	product, err := m.MkArithmeticMul(x, x)
	equationMust(t, err)
	formula, err := m.MkEq(product, x)
	equationMust(t, err)
	fixDLPin(t, m, formula)
	g := fixDLGoal(t, m, false, false, false, formula)
	count, err := (&nla2BVInvocation{t: r}).countMultiplications(g)
	equationMust(t, err)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, r.Apply(g, &result))
	if count != 1 || w.writes == 0 || !tags.IsStreamFailure(io.ErrClosedPipe) {
		t.Fatal("trace refusal changed retained computation", count, w.writes)
	}
	// The sticky stream failure is diagnostic state, not a tactic exception.
	if errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("trace refusal escaped computation")
	}
}
