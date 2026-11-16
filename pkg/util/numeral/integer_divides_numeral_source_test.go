package numeral

// Source: src/util/mpz.cpp:2054-2069; src/util/scoped_numeral.h:31-33.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type integerDividesReentryFixture struct {
	manager  *IntegerManager
	finalize bool
	called   bool
	changed  bool
	result   error
	context  integerAllocationInvocationContext
}

var integerDividesReentryCurrent *integerDividesReentryFixture

func integerDividesReentryTransport(f umemory.Failure) error {
	s := integerDividesReentryCurrent
	if s != nil && !s.called {
		s.called = true
		m := s.manager
		s.context = integerAllocationInvocationContext{BoolDivides: &m.DividesNumeral.Receipt, BoolDividesInvocation: m.DividesNumeral.Invocation}
		stage, finalizing, finalized, cause := m.SourceFinalizeStage, m.SourceFinalizing, m.SourceFinalized, m.CleanupCauseField
		if s.finalize {
			s.result = m.SourceFinalize()
		} else {
			s.result = m.CloseChecked()
		}
		s.changed = stage != m.SourceFinalizeStage || finalizing != m.SourceFinalizing || finalized != m.SourceFinalized || cause != m.CleanupCauseField
	}
	return f
}
func TestIntegerDividesScopedNUMExecutingOwnerRefusal(t *testing.T) {
	for _, finalize := range []bool{true, false} {
		budget := arena.NewBudget(1 << 24)
		rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, Transport: integerDividesReentryTransport})
		serviceMust(t, e)
		cfg := serviceConfig(budget).Integers
		cfg.Memory = rt
		cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
		m, e := NewIntegerManager(cfg)
		serviceMust(t, e)
		var a, b Int
		digits := make([]uint32, 64)
		digits[0] = 3
		digits[63] = 1
		serviceMust(t, m.SetLimbs(&b, digits))
		a = NewInt(7)
		fixture := integerDividesReentryFixture{manager: m, finalize: finalize}
		integerDividesReentryCurrent = &fixture
		budget.FailAfter(0)
		_, body := m.Divides(&a, &b)
		integerDividesReentryCurrent = nil
		if !fixture.called || body == nil || !errors.Is(fixture.result, ErrNumericPending) || fixture.changed {
			t.Fatalf("executing scoped NUM owner refusal: %+v body=%v", fixture, body)
		}
		integerPhysicalAudit(t, m, &a, &b, &m.DividesNumeral.Numeral)
		budget.FailAfter(-1)
		serviceMust(t, m.CleanupDividesNumeralInvocation(&m.DividesNumeral.Receipt, false))
		serviceMust(t, m.CleanupDividesNumeralInvocation(&m.DividesNumeral.Receipt, true))
		serviceMust(t, m.Del(&b))
		serviceMust(t, m.CloseChecked())
		serviceMust(t, rt.Close())
	}
}

func TestIntegerDividesScopedNUMBeforeZeroBranch(t *testing.T) {
	m, _ := integerManager(t)
	zero, one := NewInt(0), NewInt(1)
	result, e := m.dividesWithAllocationContext(&zero, &one, numeralContextRef(integerAllocationInvocationContext{}))
	serviceMust(t, e)
	s := &m.DividesNumeral
	if result || s.Invocation == 0 || s.Receipt.Manager != m || s.Receipt.Frame != s || s.Receipt.Invocation != s.Invocation || !s.Constructed || !s.Ended || s.Active || s.Numeral != (Int{}) {
		t.Fatal("literal scoped tmp must construct before zero branch and DT on exit", result, s)
	}
	previous := s.Invocation
	result, e = m.dividesWithAllocationContext(&zero, &zero, numeralContextRef(integerAllocationInvocationContext{}))
	serviceMust(t, e)
	if !result || s.Invocation <= previous || !s.Ended {
		t.Fatal("literal zero divides zero/new actual scoped invocation", result, s.Invocation)
	}
}

func TestIntegerDividesScopedNUMDestructorFailureClearsTrue(t *testing.T) {
	m, _ := integerManager(t)
	r, e := m.BeginDividesNumeralInvocation(integerAllocationInvocationContext{})
	serviceMust(t, e)
	s := r.Frame
	serviceMust(t, m.SetLimbs(&s.Numeral, []uint32{1, 2, 3, 4}))
	if s.Numeral.Cell == 0 {
		t.Fatal("fixture did not retain physical integer storage")
	}
	cell := s.Numeral.Cell
	s.Numeral.Cell = ^NumericCellRef(0)
	result := true // the divisibility body completed before scoped_mpz destruction
	var cause error
	m.completeDividesNumeral(s, &result, &cause)
	if result || !errors.Is(cause, arena.ErrInvalidSlot) || !s.Active || s.sourceCause == nil || s.Executing {
		t.Fatal("failed scoped_mpz destructor published divisibility", result, cause, *s)
	}
	s.Numeral.Cell = cell
	serviceMust(t, m.CleanupDividesNumeralInvocation(r, true))
	if s.Active {
		t.Fatal("owning cleanup retained completed divides invocation")
	}
}

// Invalid Go descriptor domain: authority still names only the actual guarded
// primitive frame, even when a saved pointer addresses the reused receipt.
func TestIntegerDividesScopedNUMCopiedAndRetargetedContextsRefuse(t *testing.T) {
	m, _ := integerManager(t)
	r, e := m.BeginDividesNumeralInvocation(integerAllocationInvocationContext{})
	serviceMust(t, e)
	old := integerAllocationInvocationContext{BoolDivides: r, BoolDividesInvocation: r.Invocation}
	if old.roots(m) != &r.Frame.Numeric {
		t.Fatal("current actual scoped invocation")
	}
	copyFrame := *r.Frame
	copied := integerDividesNumeralInvocationReceipt{m, &copyFrame, r.Invocation}
	before := r.Frame.Numeral
	if m.dividesNumeralContextRoots(&integerAllocationInvocationContext{BoolDivides: &copied, BoolDividesInvocation: copied.Invocation}) != nil {
		t.Fatal("copied child accepted")
	}
	if !errors.Is(m.FinishDividesNumeralSource(&copyFrame), ErrNumericOwner) {
		t.Fatal("copied source frame accepted")
	}
	serviceMust(t, m.CleanupDividesNumeralInvocation(&copied, true))
	if r.Frame.Numeral != before || !r.Frame.Active {
		t.Fatal("copied child mutated real scope")
	}
	r.Frame.Executing = false
	serviceMust(t, m.FinishDividesNumeralSource(r.Frame))
	next, e := m.BeginDividesNumeralInvocation(integerAllocationInvocationContext{})
	serviceMust(t, e)
	current := integerAllocationInvocationContext{BoolDivides: next, BoolDividesInvocation: next.Invocation}
	if old.BoolDivides != current.BoolDivides || old.BoolDividesInvocation == current.BoolDividesInvocation || old.roots(m) != nil || current.roots(m) != &next.Frame.Numeric {
		t.Fatal("saved embedded pointer retargeted invocation")
	}
	next.Frame.Executing = false
	serviceMust(t, m.FinishDividesNumeralSource(next.Frame))
}

// Actual signed-host TLS guard domain, not ordinary mathematical exhaustion.
func TestIntegerDividesScopedNUMSourceDTAndImmutableHistory(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Memory = rt
	m, e := NewIntegerManager(cfg)
	serviceMust(t, e)
	var a, d Int
	words := make([]uint32, 64)
	words[63] = 2
	serviceMust(t, m.SetLimbs(&a, words))
	words[0] = 7
	words[63] = 3
	serviceMust(t, m.SetLimbs(&d, words))
	_, e = m.Divides(&a, &d)
	serviceMust(t, e)
	serviceMust(t, m.EnsureTmp(0, 64))
	serviceMust(t, m.EnsureTmp(1, 64))
	contextTLSNearMinimum(t, rt)
	lane, e := rt.Allocate(8)
	serviceMust(t, e)
	delta := int64(-99900) - rt.State().LocalSize
	if e = rt.Reallocate(&lane, uint64(16)+uint64(delta)-8); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("actual signed allocation guard", e)
	}
	result, returned := m.Divides(&a, &d)
	s := &m.DividesNumeral
	r := s.Receipt
	if returned == nil || !errors.Is(returned, arena.ErrBounds) || result || s.sourceCause == nil || s.BodyCause != nil {
		t.Fatal("literal completed Rem then scoped NUM DT stop", result, returned, s.BodyCause, s.sourceCause, "num", s.Ended, s.Numeral)
	}
	captured, source := returned.Error(), s.sourceCause
	integerPhysicalAuditWithRaw(t, m, []umemory.Block{lane}, &a, &d, &s.Numeral)
	if b.Used() != integerActualOwnedBytes(m) {
		t.Fatal("immediate named cold graph", b.Used(), integerActualOwnedBytes(m))
	}
	own := m.CleanupDividesNumeralInvocation(&r, false)
	if own != nil && !errors.Is(own, arena.ErrBounds) {
		t.Fatal("false own suffix", own)
	}
	if m.DividesNumeralInvocationSourceCause(&r) != source || m.FinishDividesNumeralSource(s) != source {
		t.Fatal("first actual source DT cause lost after own suffix")
	}
	recovery, e := rt.Allocate(2000000)
	serviceMust(t, e)
	serviceMust(t, m.CleanupDividesNumeralInvocation(&r, true))
	if m.DividesNumeralInvocationSourceCause(&r) != source || returned.Error() != captured {
		t.Fatal("own ack changed source history")
	}
	zero := NewInt(0)
	_, e = m.Divides(&zero, &zero)
	serviceMust(t, e)
	if s.Invocation == r.Invocation || returned.Error() != captured || !errors.Is(returned, arena.ErrBounds) {
		t.Fatal("later actual invocation retargeted returned cause")
	}
	serviceMust(t, m.Del(&a))
	serviceMust(t, m.Del(&d))
	serviceMust(t, rt.Deallocate(&lane))
	serviceMust(t, m.CloseChecked())
	e = rt.Deallocate(&recovery)
	if e != nil && (!errors.Is(e, arena.ErrBounds) || recovery != (umemory.Block{})) {
		t.Fatal("recovery consumed outcome", e)
	}
	contextTLSFinish(t, rt, rt.Close)
	if b.Used() != 0 {
		t.Fatal("explicit complete owner roots", b.Used())
	}
}

func TestIntegerDividesScopedNUMEveryActualRemPrefix(t *testing.T) {
	success, acquired := false, false
	for prefix := int64(0); prefix < 256 && !success; prefix++ {
		m, budget := mpzPublicationOwner(t)
		a, b := Int{}, Int{}
		modulus := make([]uint32, 513)
		modulus[512] = 1
		dividend := make([]uint32, 640)
		dividend[0] = 1
		dividend[500] = 1
		dividend[639] = 1
		serviceMust(t, m.SetLimbs(&a, modulus))
		serviceMust(t, m.SetLimbs(&b, dividend))
		budget.FailAfter(prefix)
		result, body := m.dividesWithAllocationContext(&a, &b, numeralContextRef(integerAllocationInvocationContext{}))
		s := &m.DividesNumeral
		integerPhysicalAudit(t, m, &a, &b, &s.Numeral)
		if body != nil && !numericPhysicalReservationRefusal(body) {
			t.Fatal("actual Rem/source NUM DT", prefix, body)
		}
		if NumericAllocationPending(&s.Numeric) {
			acquired = true
			if s.Receipt.Frame != s || s.Receipt.Invocation != s.Invocation || !s.Active {
				t.Fatal("actual pending leaf missing scoped NUM tuple", prefix)
			}
		}

		budget.FailAfter(-1)
		serviceMust(t, m.CleanupDividesNumeralInvocation(&s.Receipt, false))
		serviceMust(t, m.CleanupDividesNumeralInvocation(&s.Receipt, true))
		integerPhysicalAudit(t, m, &a, &b)
		if body == nil {
			if result {
				t.Fatal("independent nonzero remainder", prefix)
			}
			success = true
			t.Log("actual Rem first success", prefix)
		}
		serviceMust(t, m.Del(&a))
		serviceMust(t, m.Del(&b))
	}
	if !success || !acquired {
		t.Fatal("actual pending/first success not reached", success, acquired)
	}
}
