package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1767-1797;
// src/util/memory_manager.cpp:274-285. Returning-transport lifecycle fixture.

import (
	"math"
	"reflect"
	"testing"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestUPolynomialEvalSignRationalActualCallbackOwnerAndTupleRefusal(t *testing.T) {
	var c *AlgebraicRootCore
	var p *UPolynomialManager
	armed := false
	calls := 0
	transport := func(f umemory.Failure) error {
		if !armed {
			return f
		}
		calls++
		q := &p.rationalEvaluation
		before := *q
		z := p.config.Integer
		used := z.Budget().Used()
		physical := z.RuntimeOwner().Stats()
		r := p.rationalEvaluationReceipt()

		actual := q.context()
		if actual.rationalEvaluationFrame(z) != nil || actual.roots(z) != nil {
			t.Error("failure transport acquired paused rational Horner context")
		}
		probe := unumeral.NewInt(0)
		if e := z.WithAllocationContext(actual.utility(z)).SetUint32(&probe, 11); e == nil || probe.Val != 0 || probe.Cell != 0 {
			t.Error("failure transport wrote through rational Horner tuple", e, probe)
		}
		inflight := unumeral.NumericAllocationContext{}.WithCaller(&q.numeric.hooks)
		if e := z.WithAllocationContext(inflight).SetUint32(&probe, 12); e == nil || probe.Val != 0 || probe.Cell != 0 {
			t.Error("failure transport reused in-flight rational Horner policy", e, probe)
		}
		copied := *q
		ctx := q.context()
		ctx.rationalEvaluation = &copied
		if (ctx.roots(z)) != nil {
			t.Error("copied frame acquired numeric leaf")
		}
		stale := q.context()
		stale.rationalEvaluationInvocation--
		if (stale.roots(z)) != nil {
			t.Error("stale invocation acquired numeric leaf")
		}
		owner := *p
		owner.self = &owner
		owner.rationalEvaluation.owner = &owner
		if (integerAllocationContextRef(owner.rationalEvaluation.context()).roots(z)) != nil {
			t.Error("copied owner acquired numeric leaf")
		}
		foreign := r
		foreign.frame = &copied
		serviceMust(t, p.cleanupRationalEvaluationWithReceipt(&foreign, true))
		for _, abort := range []bool{false, true} {
			if e := p.cleanupRationalEvaluationWithReceipt(&r, abort); e != ErrUPolynomialCleanupPending {
				t.Errorf("executing exact cleanup %v", e)
			}
		}
		if e := q.finish(nil); e != ErrUPolynomialCleanupPending {
			t.Errorf("executing source finalize %v", e)
		}
		if e := p.SourceFinalize(); e != ErrUPolynomialCleanupPending {
			t.Errorf("UP source owner %v", e)
		}
		if e := p.Close(); e != ErrUPolynomialCleanupPending {
			t.Errorf("UP owning owner %v", e)
		}
		if e := z.SourceFinalize(); e != ErrPolynomialPending {
			t.Errorf("Integer source owner %v", e)
		}
		if e := z.CloseChecked(); e != ErrPolynomialPending {
			t.Errorf("Integer own owner %v", e)
		}
		if e := c.config.Rationals.SourceFinalize(); e != ErrPolynomialPending {
			t.Errorf("Rational source owner %v", e)
		}
		if e := c.config.Rationals.CloseChecked(); e != ErrPolynomialPending {
			t.Errorf("Rational own owner %v", e)
		}
		if !reflect.DeepEqual(before, *q) || used != z.Budget().Used() || physical != z.RuntimeOwner().Stats() {
			t.Error("refused callback changed exact charged frame/physical graph")
		}
		return f
	}
	var rt *umemory.Runtime
	c, p, rt = algebraicNegTLSFixture(t, transport)
	poly := algebraicPolynomial(t, p, 1, math.MaxInt32, 1)
	v, e := p.wholeView(poly)
	serviceMust(t, e)
	var b unumeral.Rational
	serviceMust(t, c.config.Rationals.SetInt32(&b, math.MaxInt32))
	held, e := rt.Allocate(uint64(int64(100000) - rt.State().LocalSize - 12))
	serviceMust(t, e)
	serviceMust(t, rt.SetMaxSize(1))
	armed = true
	_, body := p.EvalSignRational(v, &b)
	armed = false
	if body == nil || calls == 0 {
		t.Fatal("actual allocating Horner transport not reached", body, calls)
	}
	q := &p.rationalEvaluation
	extra := []*unumeral.Int{&(*b.BorrowNumerator()), &(*b.BorrowDenominator()), &q.values[0], &q.values[1], &q.values[2]}
	if q.addMul.Frame != nil {
		extra = append(extra, &q.addMul.Frame.Tmp)
	}
	algebraicCellSourcePhysical(t, c, []UPolynomial{poly}, extra, []umemory.Block{held})
	serviceMust(t, rt.SetMaxSize(0))
	serviceMust(t, p.cleanupRationalEvaluation(false))
	serviceMust(t, c.config.Rationals.Del(&b))
	serviceMust(t, p.cleanupRationalEvaluation(true))
	serviceMust(t, rt.Deallocate(&held))
}
