package polynomial

// Source: src/math/polynomial/upolynomial.cpp:3042-3118;
// src/util/mpzzp.h:58-68; src/util/mpn.cpp:197-215.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

// Qualified signed-host TLS release guard; no child or cursor is seeded.
func TestUPolynomialFactorFrontendActualOuterVectorSourceStop(t *testing.T) {
	if !polynomialSourceDebug {
		return
	}
	budget := arena.NewBudget(1 << 26)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	serviceMust(t, e)
	cfg := serviceConfig(budget).Integers
	cfg.Memory = rt
	cfg.Cells.PageSize = 1
	z, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	m, e := NewUPolynomialManager(UPolynomialConfig{Integer: z.Integers(), Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }})
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(budget)
	serviceMust(t, e)
	s, e := NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
	serviceMust(t, e)
	fs, e := m.NewFactors()
	serviceMust(t, e)
	var input, later upolyVectorState
	var b unumeral.Int
	words := make([]uint32, 17)
	words[16] = 128
	serviceMust(t, z.Integers().SetLimbs(&b, words))
	serviceMust(t, m.set(&input, []unumeral.Int{unumeral.NewInt(1), b, unumeral.NewInt(1)}))
	serviceMust(t, m.set(&later, []unumeral.Int{unumeral.NewInt(-1), unumeral.NewInt(0), unumeral.NewInt(1)}))
	roots := []*unumeral.Int{&b}
	for i := range m.values(&input) {
		roots = append(roots, &m.values(&input)[i])
	}
	// Warm actual persistent scratch before the later source-stop call.
	_, e = s.Factor(m.stateView(&input), fs, DefaultUPolynomialFactorParams())
	serviceMust(t, e)
	serviceMust(t, fs.Reset())
	external, e := rt.Allocate(200000)
	serviceMust(t, e)
	var bodyRecovery umemory.Block
	t.Cleanup(func() {
		recovery, x := rt.Allocate(4000000)
		serviceMust(t, x)
		contextTLSFinish(t, rt, s.CleanupRetained)
		contextTLSFinish(t, rt, fs.Close)
		contextTLSFinish(t, rt, func() error { return m.reset(&input) })
		m.freeBacking(&input)
		contextTLSFinish(t, rt, func() error { return m.reset(&later) })
		m.freeBacking(&later)
		contextTLSFinish(t, rt, func() error { return z.Integers().Del(&b) })
		contextTLSFinish(t, rt, s.Close)
		t.Log("after service", rt.Stats())
		contextTLSFinish(t, rt, primes.Close)
		contextTLSFinish(t, rt, m.Close)
		t.Log("after UPM", rt.Stats())
		contextTLSFinish(t, rt, z.CloseChecked)
		t.Log("after Rational", rt.Stats())
		if bodyRecovery != (umemory.Block{}) {
			contextTLSFinish(t, rt, func() error { return rt.Deallocate(&bodyRecovery) })
		}
		if external != (umemory.Block{}) {
			contextTLSFinish(t, rt, func() error { return rt.Deallocate(&external) })
		}
		x = rt.Deallocate(&recovery)
		if x != nil && (!errors.Is(x, arena.ErrBounds) || recovery != (umemory.Block{})) {
			t.Fatal(x)
		}
		t.Log("before Runtime", rt.Stats())
		contextTLSFinish(t, rt, rt.Close)
		if budget.Used() != 0 {
			t.Fatal("actual owner refunds", budget.Used())
		}
	})
	armed := false
	var release error
	m.config.Checkpoint = func() bool {
		if !armed {
			if !s.frontend.image.classConstructed || s.frontend.modularGCD.constructed != 6 {
				t.Fatal("actual checkpoint constructor frontier")
			}
			armed = true
			contextTLSNearMinimum(t, rt)
			release = rt.Deallocate(&external)
		}
		return true
	}
	_, body := s.Factor(m.stateView(&input), fs, DefaultUPolynomialFactorParams())
	primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{fs}, nil, roots...)
	if !armed || !errors.Is(release, arena.ErrBounds) || external != (umemory.Block{}) {
		t.Fatal("actual checkpoint setup release", armed, release)
	}
	oldTmp := z.Integers().Tmp[0]
	if oldTmp == 0 || !z.Integers().Cell(oldTmp).Released {
		t.Fatal("actual physically consumed descriptor frontier", oldTmp)
	}
	for i, ref := range z.Integers().Tmp {
		if ref != 0 {
			c := z.Integers().Cell(ref)
			t.Logf("actual shared Integer tmp[%d] ref=%v capacity=%d released=%v backing=%+v", i, ref, c.Capacity, c.Released, c.Backing)
		}
	}
	q := &s.frontend
	t.Logf("body=%v source=%v constructed=%d image=%v/%v modGCD=%v/%d", body, q.sourceCause, q.constructed, q.image.classConstructed, q.image.savedConstructed, q.modularGCD.executing, q.modularGCD.constructed)
	if body == nil || q.sourceCause == nil || !q.active {
		t.Fatal("actual reached source stop missing", body, q.sourceCause, q.active)
	}
	if q.image.sourceCause != nil || q.modularGCD.sourceCause != nil || q.sourceStoppedStage != 3 || q.constructed != 3 {
		t.Fatal("real outer prime vector frontier after completed younger child DT", q.sourceStoppedStage, q.constructed)
	}
	original, cause, receipt := body, q.sourceCause, q.receipt
	count, stage, state := q.constructed, q.sourceStoppedStage, rt.State()
	if e = q.finish(nil); e == nil || q.constructed != count || q.sourceStoppedStage != stage || rt.State() != state {
		t.Fatal("source retry replayed older suffix", e)
	}
	bodyRecovery, e = rt.Allocate(400000)
	serviceMust(t, e)
	contextTLSFinish(t, rt, func() error { return s.cleanupFactorFrontendInvocation(&receipt, false) })
	contextTLSFinish(t, rt, func() error { return s.cleanupFactorFrontendInvocation(&receipt, true) })
	if q.sourceCause != cause || !errors.Is(original, cause) {
		t.Fatal("own acknowledgment changed source history")
	}
	primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{fs}, []umemory.Block{bodyRecovery}, roots...)
	oldHistory := *receipt.history
	serviceMust(t, fs.Reset())
	// The scratch whose release stopped is finished and replaced by the next
	// ensure_tmp once memory is back (z3's deallocate cannot fail).
	_, laterBIG := s.Factor(m.stateView(&input), fs, DefaultUPolynomialFactorParams())
	if laterBIG != nil || z.Integers().Tmp[0] == oldTmp {
		t.Fatal("repeated BIG use did not repair the stopped scratch release", laterBIG)
	}
	serviceMust(t, s.CleanupRetained())
	serviceMust(t, fs.Reset())
	// A later inline-coefficient route does not reuse the failed Natural scratch.
	_, e = s.Factor(m.stateView(&later), fs, DefaultUPolynomialFactorParams())
	serviceMust(t, e)
	if receipt.history.sourceCause != cause || receipt.history.bodyCause != oldHistory.bodyCause || receipt.history.sourceStoppedStage != oldHistory.sourceStoppedStage || !errors.Is(original, cause) {
		t.Fatal("new actual call erased archived first stop")
	}
	before := rt.State()
	current := s.frontend
	serviceMust(t, s.cleanupFactorFrontendInvocation(&receipt, true))
	if before != rt.State() || current.invocation != s.frontend.invocation || current.receipt != s.frontend.receipt {
		t.Fatal("old completed ACK touched later invocation")
	}
	primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{fs}, []umemory.Block{bodyRecovery}, roots...)
	t.Log("body before recovery free", bodyRecovery, rt.Stats())
	e = rt.Deallocate(&bodyRecovery)
	t.Log("body after recovery free", bodyRecovery, e, rt.Stats())
	if e != nil && (!errors.Is(e, arena.ErrBounds) || bodyRecovery != (umemory.Block{})) {
		t.Fatal(e)
	}
}
