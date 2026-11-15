package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1030-1050;
// src/util/mpz.h:124-149; src/util/memory_manager.cpp:274-285.

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

func TestUPolynomialFactorPreparationActualCopyTransport(t *testing.T) {
	for _, mode := range []string{"service", "close", "source", "mode"} {
		t.Run(mode, func(t *testing.T) { preparationActualCopyTransport(t, mode) })
	}
}
func preparationActualCopyTransport(t *testing.T, mode string) {
	b := arena.NewBudget(1 << 26)
	var s *UPolynomialFactorizationService
	armed := false
	calls := 0
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			prior := s.scope
			before := *prior
			var stop error
			switch mode {
			case "service":
				stop = s.CleanupRetained()
			case "close":
				stop = s.manager.Close()
			case "source":
				stop = s.manager.SourceFinalize()
			case "mode":
				stop = s.manager.SetZ()
			}
			if stop != ErrUPolynomialCleanupPending || s.scope != prior || !reflect.DeepEqual(*prior, before) {
				t.Errorf("actual preparation coefficient SET caller changed before refusal: %v", stop)
			}
		}
		return f
	}})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	m, e := NewUPolynomialManager(UPolynomialConfig{Integer: q.Integers(), Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }})
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(b)
	serviceMust(t, e)
	s, e = NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
	serviceMust(t, e)
	var held umemory.Block
	fs, e := m.NewFactors()
	serviceMust(t, e)
	var value unumeral.Int
	defer func() {
		armed = false
		b.FailAfter(-1)
		serviceMust(t, rt.SetMaxSize(0))
		contextTLSFinish(t, rt, s.CleanupRetained)
		contextTLSFinish(t, rt, fs.Close)
		contextTLSFinish(t, rt, func() error { return q.Integers().Del(&value) })
		contextTLSFinish(t, rt, func() error { return rt.Deallocate(&held) })
		contextTLSFinish(t, rt, s.Close)
		serviceMust(t, primes.Close())
		contextTLSFinish(t, rt, m.Close)
		contextTLSFinish(t, rt, q.CloseChecked)
		contextTLSFinish(t, rt, rt.Close)
		if b.Used() != 0 {
			t.Error("actual coefficient SET owner refunds", b.Used())
		}
	}()
	parent, e := s.newSquareFreeScope()
	serviceMust(t, e)
	parent.constructed = 1 // actual f_pp vector declaration; no p/related fields yet
	words := make([]uint32, 80)
	words[79] = 1
	serviceMust(t, q.Integers().SetLimbs(&value, words))
	var original upolyVectorState
	serviceMust(t, m.set(&original, []unumeral.Int{value, unumeral.NewInt(1)}))
	serviceMust(t, fs.PushCopy(m.stateView(&original), 1))
	serviceMust(t, m.reset(&original))
	m.freeBacking(&original)
	state, e := fs.factor(0)
	serviceMust(t, e)
	// Warm only the literal vector reservation, isolating the next actual BIG
	// coefficient SET allocator leaf rather than a preceding vector request.
	serviceMust(t, m.reserve(&parent.fpp, 2))
	pad := int64(99996) - rt.State().LocalSize - 12
	if pad <= 0 {
		t.Fatal("actual source threshold", rt.State())
	}
	held, e = rt.Allocate(uint64(pad))
	serviceMust(t, e)
	serviceMust(t, rt.SetMaxSize(1))
	armed = true
	body := s.prepareSquareFree(m.stateView(state), fs, 1)
	armed = false
	if !numericPhysicalReservationRefusal(body) || calls != 1 {
		t.Fatal("actual source coefficient SET refusal", body, calls)
	}
	primeTrialPhysicalAuditAll(t, q, s, []*UPolynomialFactors{fs}, []umemory.Block{held}, &value)
}

// Actual DEBUG helper before the enclosing f_pp declaration; no parent
// owner or numeral is constructed early to grant acquisition authority.
func TestUPolynomialFactorPreparationDiagnosticBeforeParentTransport(t *testing.T) {
	for _, mode := range []string{"service", "close", "source", "mode"} {
		t.Run(mode, func(t *testing.T) { preparationDiagnosticBeforeParentTransport(t, mode) })
	}
}
func preparationDiagnosticBeforeParentTransport(t *testing.T, mode string) {
	b := arena.NewBudget(1 << 26)
	var s *UPolynomialFactorizationService
	armed := false
	calls := 0
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			before := *s
			var stop error
			switch mode {
			case "service":
				stop = s.Close()
			case "close":
				stop = s.manager.Close()
			case "source":
				stop = s.manager.SourceFinalize()
			case "mode":
				stop = s.manager.SetZ()
			}
			if stop != ErrUPolynomialCleanupPending || !reflect.DeepEqual(*s, before) {
				t.Errorf("actual preparation coefficient SET caller changed before refusal: %v", stop)
			}
		}
		return f
	}})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	m, e := NewUPolynomialManager(UPolynomialConfig{Integer: q.Integers(), Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }})
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(b)
	serviceMust(t, e)
	s, e = NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
	serviceMust(t, e)
	var held umemory.Block
	fs, e := m.NewFactors()
	serviceMust(t, e)
	var value unumeral.Int
	defer func() {
		armed = false
		b.FailAfter(-1)
		serviceMust(t, rt.SetMaxSize(0))
		contextTLSFinish(t, rt, s.CleanupRetained)
		contextTLSFinish(t, rt, fs.Close)
		contextTLSFinish(t, rt, func() error { return q.Integers().Del(&value) })
		contextTLSFinish(t, rt, func() error { return rt.Deallocate(&held) })
		if !s.closed {
			contextTLSFinish(t, rt, s.Close)
		}
		serviceMust(t, primes.Close())
		contextTLSFinish(t, rt, m.Close)
		contextTLSFinish(t, rt, q.CloseChecked)
		contextTLSFinish(t, rt, rt.Close)
		if b.Used() != 0 {
			t.Error("actual coefficient SET owner refunds", b.Used())
		}
	}()
	words := make([]uint32, 80)
	words[79] = 1
	serviceMust(t, q.Integers().SetLimbs(&value, words))
	var original upolyVectorState
	serviceMust(t, m.set(&original, []unumeral.Int{value, unumeral.NewInt(1)}))
	serviceMust(t, fs.PushCopy(m.stateView(&original), 1))
	serviceMust(t, m.reset(&original))
	m.freeBacking(&original)
	state, e := fs.factor(0)
	serviceMust(t, e)
	// The DEBUG scoped content precedes f_pp and every enclosing source field.
	if s.scope != nil {
		t.Fatal("diagnostic constructed parent early")
	}
	pad := int64(99996) - rt.State().LocalSize - 12
	if pad <= 0 {
		t.Fatal("actual source threshold", rt.State())
	}
	held, e = rt.Allocate(uint64(pad))
	serviceMust(t, e)
	serviceMust(t, rt.SetMaxSize(1))
	armed = true
	body := s.primitiveDiagnosticWithPreparation(m.stateView(state))
	armed = false
	if !numericPhysicalReservationRefusal(body) || calls != 1 {
		t.Fatal("actual source coefficient SET refusal", body, calls)
	}
	primeTrialPhysicalAuditAll(t, q, s, []*UPolynomialFactors{fs}, []umemory.Block{held}, &value)
}

// Primitive cubic source input, caller content BIG, and every actual
// reservation prefix through conditional c publication and source destruction.
func TestUPolynomialFactorPreparationConditionalEveryActualPrefix(t *testing.T) {
	for _, negative := range []bool{false, true} {
		for _, k := range []uint32{1, 2} {
			completed := false
			for ordinal := int64(0); ordinal < 512; ordinal++ {
				done := false
				t.Run(fmt.Sprintf("%v/%d/%d", negative, k, ordinal), func(t *testing.T) {
					q, s := mignotteDirectFixture(t)
					m := s.manager
					fs, e := m.NewFactors()
					serviceMust(t, e)
					var large, constant, want unumeral.Int
					serviceMust(t, q.Integers().SetLimbs(&large, []uint32{0, 0, 2}))
					serviceMust(t, q.Integers().SetLimbs(&want, []uint32{0, 0, 2}))
					serviceMust(t, q.Integers().Copy(&constant, &q.Integers().Two64))
					serviceMust(t, fs.SetConstant(&constant))
					if negative {
						serviceMust(t, q.Integers().Neg(&large))
					}
					small := unumeral.NewInt(1)
					if negative {
						small = unumeral.NewInt(-1)
					}
					var original upolyVectorState
					serviceMust(t, m.set(&original, []unumeral.Int{small, unumeral.NewInt(0), unumeral.NewInt(0), large}))
					serviceMust(t, fs.PushCopy(m.stateView(&original), 1))
					serviceMust(t, m.reset(&original))
					m.freeBacking(&original)
					originalState, e := fs.factor(0)
					serviceMust(t, e)
					view := m.stateView(originalState)
					parent, e := s.newSquareFreeScope()
					serviceMust(t, e)
					parent.constructed = 1
					q.Budget().FailAfter(ordinal)
					body := s.prepareSquareFree(view, fs, k)
					primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &large, &constant, &want)
					h := &s.preparation
					r := h.receipt
					source, own := h.sourceCause, h.ownCause
					if body == nil {
						expected := unumeral.NewInt(1)
						actual, e := m.read(m.stateView(&parent.fpp), 0)
						serviceMust(t, e)
						same, e := q.Integers().Equal(actual, &expected)
						serviceMust(t, e)
						if !same {
							t.Fatal("source sign-loop independent constant", actual)
						}
						top, e := m.read(m.stateView(&parent.fpp), 3)
						serviceMust(t, e)
						same, e = q.Integers().Equal(top, &want)
						serviceMust(t, e)
						if !same {
							t.Fatal("source sign-loop independent leading coefficient")
						}
						sign := int32(1)
						if negative && k%2 == 1 {
							sign = -1
						}
						positive, e := q.Integers().IsNeg(&fs.constant)
						serviceMust(t, e)
						if positive != (sign < 0) {
							t.Fatal("actual caller constant publication", negative, k, fs.constant)
						}
						done = true
					} else if !numericPhysicalReservationRefusal(body) {
						t.Fatal("actual source reservation", body)
					}
					if (!negative || k%2 == 0) && h.numeral != (unumeral.Int{}) {
						t.Fatal("erased c branch acquired numeral")
					}
					q.Budget().FailAfter(-1)
					serviceMust(t, h.own(&r, false))
					serviceMust(t, s.CleanupRetained())
					if h.active || h.bodyCause != body || h.sourceCause != source || h.ownCause != own {
						t.Fatal("exact preparation history/retirement", h.active, h.bodyCause, body)
					}
					primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &large, &constant, &want)
					serviceMust(t, fs.Close())
					serviceMust(t, q.Integers().Del(&large))
					serviceMust(t, q.Integers().Del(&constant))
					serviceMust(t, q.Integers().Del(&want))
					primeTrialPhysicalAudit(t, q, s)
				})
				if done {
					completed = true
					t.Log("first complete", negative, k, ordinal)
					break
				}
			}
			if !completed {
				t.Fatal("no actual source completion", negative, k)
			}
		}
	}
}

// Real declared conditional NUM and actual source/own release; signed-host TLS
// guard composition isolates the destructor, not ordinary math OOM reachability.
func TestUPolynomialFactorPreparationActualConditionalDestructorHistory(t *testing.T) {
	for _, source := range []bool{true, false} {
		t.Run(fmt.Sprint(source), func(t *testing.T) {
			b := arena.NewBudget(1 << 26)
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true})
			serviceMust(t, e)
			cfg := serviceConfig(b).Integers
			cfg.Memory = rt
			cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
			zq, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
			serviceMust(t, e)
			m, e := NewUPolynomialManager(UPolynomialConfig{Integer: zq.Integers(), Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }})
			serviceMust(t, e)
			primes, e := primegen.NewGenerator(b)
			serviceMust(t, e)
			s, e := NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
			serviceMust(t, e)
			fs, e := m.NewFactors()
			serviceMust(t, e)
			parent, e := s.newSquareFreeScope()
			serviceMust(t, e)
			parent.constructed = 1
			serviceMust(t, m.set(&parent.fpp, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(1)}))
			h, e := s.beginFactorPreparation(false)
			serviceMust(t, e)
			r := h.receipt
			context := numericIntegerModularManager_preparationAllocationContext(m.modular, h)
			h.constructed = true
			serviceMust(t, m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(&h.numeral, &zq.Integers().Two64))
			serviceMust(t, zq.Integers().EnsureOwnedCapacity(&h.numeral, 40000))
			h.executing = false
			original := parent.fpp
			contextTLSNearMinimum(t, rt)
			blocks := rt.Stats().LiveBlocks
			var first error
			if source {
				first = h.finish(&r, nil)
			} else {
				first = h.own(&r, false)
			}
			if !errors.Is(first, arena.ErrBounds) || rt.Stats().LiveBlocks+1 != blocks {
				t.Fatal("actual conditional NUM release", first)
			}
			if parent.fpp != original {
				t.Fatal("younger conditional DT changed older vector")
			}
			primeTrialPhysicalAuditOwners(t, zq, s, []*UPolynomialFactors{fs})
			if source {
				before := *h
				again := s.finishSquareFreeScopeSource()
				if !errors.Is(again, h.sourceCause) || parent.fpp != original || !reflect.DeepEqual(*h, before) {
					t.Fatal("source retry replayed older suffix", again)
				}
			} else if h.ownCause != first || h.ownEnded {
				t.Fatal("first own stop")
			}
			recovery, e := rt.Allocate(400000)
			serviceMust(t, e)
			contextTLSFinish(t, rt, s.CleanupRetained)
			if source && !errors.Is(first, h.sourceCause) || !source && h.ownCause != first {
				t.Fatal("matched owning acknowledgment lost immutable history")
			}
			primeTrialPhysicalAuditAll(t, zq, s, []*UPolynomialFactors{fs}, []umemory.Block{recovery})
			contextTLSFinish(t, rt, fs.Close)
			e = rt.Deallocate(&recovery)
			if e != nil && (!errors.Is(e, arena.ErrBounds) || recovery != (umemory.Block{})) {
				t.Fatal("recovery release", e)
			}
			contextTLSFinish(t, rt, s.Close)
			serviceMust(t, primes.Close())
			contextTLSFinish(t, rt, m.Close)
			contextTLSFinish(t, rt, zq.CloseChecked)
			contextTLSFinish(t, rt, rt.Close)
			if b.Used() != 0 {
				t.Fatal("actual refunds", b.Used())
			}
		})
	}
}

// Invalid Go metadata guards and actual frame reuse are distinct from source
// allocation-prefix credit. Old context must not follow a reused member.
func TestUPolynomialFactorPreparationExactContextHistoryGuards(t *testing.T) {
	q, s := mignotteDirectFixture(t)
	parent, e := s.newSquareFreeScope()
	serviceMust(t, e)
	parent.constructed = 1
	h, e := s.beginFactorPreparation(false)
	serviceMust(t, e)
	r := h.receipt
	c := numericIntegerModularManager_preparationAllocationContext(s.manager.modular, h)
	if (c.roots(q.Integers())) != (&h.numeric) {
		t.Fatal("actual captured context missing")
	}
	before := *h
	if e = h.own(&r, false); e != ErrUPolynomialCleanupPending || !reflect.DeepEqual(*h, before) || h.ownCause != nil {
		t.Fatal("executing refusal poisoned history", e)
	}
	if e = h.finish(&r, nil); e != ErrUPolynomialCleanupPending || !reflect.DeepEqual(*h, before) {
		t.Fatal("executing source refusal changed tuple", e)
	}
	copyFrame := *h
	bad := r
	bad.frame = &copyFrame
	if e = copyFrame.own(&bad, true); e != ErrUPolynomialOwner || !reflect.DeepEqual(*h, before) {
		t.Fatal("copied frame owned actual source fields", e)
	}
	copiedContext := c
	copiedContext.preparation = &copyFrame
	if (copiedContext.roots(q.Integers())) != nil {
		t.Fatal("copied frame acquired leaf")
	}
	s.manager.callbackDepth++
	if (c.roots(q.Integers())) != nil {
		t.Fatal("callback acquired passed frame leaf")
	}
	s.manager.callbackDepth--
	other, e := NewRelatedUPolynomialManager(s.manager)
	serviceMust(t, e)
	if (integerAllocationContextRef(numericIntegerModularManager_preparationAllocationContext(other.modular, h)).roots(q.Integers())) != nil {
		t.Fatal("foreign modular receiver acquired leaf")
	}
	serviceMust(t, other.Close())
	h.executing = false
	serviceMust(t, h.finish(&r, nil))
	serviceMust(t, h.own(&r, true))
	newer, e := s.beginFactorPreparation(false)
	serviceMust(t, e)
	nr := newer.receipt
	if nr.invocation == r.invocation || (c.roots(q.Integers())) != nil {
		t.Fatal("old context followed reused actual member")
	}
	before = *newer
	if e = newer.own(&r, true); e != ErrUPolynomialOwner || !reflect.DeepEqual(*newer, before) {
		t.Fatal("stale owner changed new fields", e)
	}
	newer.executing = false
	serviceMust(t, newer.finish(&nr, nil))
	serviceMust(t, s.CleanupRetained())
	primeTrialPhysicalAudit(t, q, s)
}

// Valid primitive sparse input reaches DEBUG gcd(0,BIG) direct SET before
// the later coefficient one; old leaf ownership never adopts a replacement.
func TestUPolynomialFactorPreparationDiagnosticOldLeafPreservesForeignReplacement(t *testing.T) {
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		found := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			q, s := preparationPhysicalFixture(t)
			m := s.manager
			var prime, foreign, large, value unumeral.Int
			serviceMust(t, q.Integers().SetLimbs(&prime, []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0x7fffffff}))
			words := make([]uint32, 25000)
			words[24999] = 1
			serviceMust(t, q.Integers().SetLimbs(&large, words))
			defer func() {
				serviceMust(t, q.Integers().CleanupRawNumeral(&foreign))
				serviceMust(t, q.Integers().CleanupRawNumeral(&value))
				serviceMust(t, q.Integers().Del(&prime))
				serviceMust(t, q.Integers().Del(&large))
			}()
			fs, e := m.NewFactors()
			serviceMust(t, e)
			defer func() { serviceMust(t, fs.Close()) }()
			var original upolyVectorState
			serviceMust(t, m.set(&original, []unumeral.Int{unumeral.NewInt(0), prime, unumeral.NewInt(0), unumeral.NewInt(1)}))
			serviceMust(t, fs.PushCopy(m.stateView(&original), 1))
			serviceMust(t, m.reset(&original))
			m.freeBacking(&original)
			state, e := fs.factor(0)
			serviceMust(t, e)
			// Real held NUM consumes the slot actually freed by the source vector.
			// This is a cold owner-primitive profile, not a fabricated reservation.
			serviceMust(t, q.Integers().Copy(&value, &prime))
			q.Budget().FailAfter(ordinal / 16)
			body := s.primitiveDiagnosticWithPreparation(m.stateView(state))
			h := &s.diagnostic
			r := h.receipt

			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &foreign, &large, &value)
			owned := &h.numeric.NumericAllocationRoots
			if !unumeral.NumericAllocationPending(owned) && q.Integers().MatchesRawInvocation(&h.gcdRaw) {
				owned = &h.gcdRaw.Frame.Numeric
			}
			old := owned.Cell
			oldRoots := *owned
			if unumeral.NumericAllocationPending(owned) {
				if (old.Object != (umemory.ObjectRef{}) && q.Integers().PendingCell != old) || !numericPhysicalReservationRefusal(body) {
					t.Fatal("actual direct pending leaf absent", ordinal, body, old, q.Integers().PendingCell)
				}
				_ = h.finish(&r, body)
				serviceMust(t, h.own(&r, false))
				if *owned != oldRoots || (old.Object != (umemory.ObjectRef{}) && q.Integers().PendingCell != old) {
					t.Fatal("false phase retired unjoined raw leaf")
				}
				q.Budget().FailAfter(-1)
				serviceMust(t, q.Integers().CleanupPendingAllocation())
				// Actual independent nil-context leaf starts after the old token is consumed.
				// Search its real reservation prefixes rather than minting a pending token.
				foreignFound := false
				for next := ordinal % 16; next < ordinal%16+1; next++ {
					q.Budget().FailAfter(next)
					failure := q.Integers().Copy(&foreign, &large)
					primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &foreign, &large, &value)
					laterRaw, x := q.Integers().Allocator.PendingRoots()
					serviceMust(t, x)
					if q.Integers().PendingCell.Object != (umemory.ObjectRef{}) || laterRaw != ([3]umemory.Block{}) {
						replacement := q.Integers().PendingCell
						if (replacement.Object != (umemory.ObjectRef{}) && replacement == old) || !numericPhysicalReservationRefusal(failure) {
							t.Fatal("not a genuine later generation", next, failure, replacement, old)
						}
						q.Budget().FailAfter(-1)
						serviceMust(t, h.own(&r, true))
						afterRaw, x := q.Integers().Allocator.PendingRoots()
						serviceMust(t, x)
						if q.Integers().PendingCell != replacement || afterRaw != laterRaw {
							t.Fatal("old exact abort consumed foreign replacement", replacement, q.Integers().PendingCell)
						}
						primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &foreign, &large, &value)
						serviceMust(t, h.own(&r, true))
						afterRaw, x = q.Integers().Allocator.PendingRoots()
						serviceMust(t, x)
						if q.Integers().PendingCell != replacement || afterRaw != laterRaw {
							t.Fatal("stale old abort consumed later pending")
						}
						serviceMust(t, q.Integers().CleanupPendingAllocation())
						serviceMust(t, q.Integers().CleanupRawNumeral(&foreign))
						foreignFound = true
						found = true
						t.Log("actual old/foreign pending reservation prefixes", ordinal/16, next)
						break
					}
					q.Budget().FailAfter(-1)
					serviceMust(t, q.Integers().CleanupPendingAllocation())
					serviceMust(t, q.Integers().CleanupRawNumeral(&foreign))
				}
				if !foreignFound {
					serviceMust(t, h.own(&r, true))
				}
			} else {
				q.Budget().FailAfter(-1)
				_ = h.finish(&r, body)
				serviceMust(t, h.own(&r, false))
				serviceMust(t, h.own(&r, true))
			}
		})
		if found {
			return
		}
	}
	t.Fatal("no actual direct old descriptor prefix")
}

func preparationPhysicalFixture(t *testing.T) (*unumeral.RationalManager, *UPolynomialFactorizationService) {
	return preparationPhysicalFixtureWithSlots(t, 0)
}
func preparationPhysicalFixtureWithSlots(t *testing.T, slots uint32) (*unumeral.RationalManager, *UPolynomialFactorizationService) {
	t.Helper()
	b := arena.NewBudget(1 << 26)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Cells.PageSize = 1
	cfg.Cells.SlotLimit = slots
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	m, e := NewUPolynomialManager(UPolynomialConfig{Integer: q.Integers(), Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }})
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(b)
	serviceMust(t, e)
	s, e := NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
	serviceMust(t, e)
	t.Cleanup(func() {
		b.FailAfter(-1)
		serviceMust(t, s.Close())
		serviceMust(t, primes.Close())
		serviceMust(t, m.Close())
		serviceMust(t, q.CloseChecked())
		serviceMust(t, rt.Close())
		if b.Used() != 0 {
			t.Error("actual cold fixture refunds", b.Used())
		}
	})
	return q, s
}

// Actual DEBUG source NUM precedes f_pp; every cold reservation prefix
// names the direct/GCD child roots before both fault lift and owning rescue.
func TestUPolynomialFactorPreparationDiagnosticEveryActualPrefix(t *testing.T) {
	completed := false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		done := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			q, s := preparationPhysicalFixture(t)
			m := s.manager
			var prime, held unumeral.Int
			serviceMust(t, q.Integers().SetLimbs(&prime, []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0x7fffffff}))
			defer func() {
				serviceMust(t, q.Integers().CleanupRawNumeral(&held))
				serviceMust(t, q.Integers().Del(&prime))
			}()
			fs, e := m.NewFactors()
			serviceMust(t, e)
			defer func() { serviceMust(t, fs.Close()) }()
			var original upolyVectorState
			serviceMust(t, m.set(&original, []unumeral.Int{unumeral.NewInt(0), prime, unumeral.NewInt(0), unumeral.NewInt(1)}))
			serviceMust(t, fs.PushCopy(m.stateView(&original), 1))
			serviceMust(t, m.reset(&original))
			m.freeBacking(&original)
			st, e := fs.factor(0)
			serviceMust(t, e)
			serviceMust(t, q.Integers().Copy(&held, &prime))
			q.Budget().FailAfter(ordinal)
			body := s.primitiveDiagnosticWithPreparation(m.stateView(st))
			h := &s.diagnostic
			r := h.receipt
			firstBody, firstSource := h.bodyCause, h.sourceCause
			if s.scope != nil {
				t.Fatal("DEBUG created enclosing f_pp")
			}
			if body != nil && !numericPhysicalReservationRefusal(body) {
				t.Fatal("actual DEBUG refusal", body)
			}
			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &held)
			// Source completed NUM and exact younger children first, no enclosing locals exist.
			stop := h.own(&r, false)
			if stop != nil && !numericPhysicalReservationRefusal(stop) {
				t.Fatal("false owning phase", stop)
			}
			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &held)
			if stop == nil {
				stop = h.own(&r, true)
			}
			if stop != nil && !numericPhysicalReservationRefusal(stop) {
				t.Fatal("true owning phase", stop)
			}
			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &held)
			q.Budget().FailAfter(-1)
			serviceMust(t, h.own(&r, false))
			serviceMust(t, h.own(&r, true))
			if h.active || h.bodyCause != firstBody || h.sourceCause != firstSource {
				t.Fatal("exact immutable DEBUG history")
			}
			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &held)
			done = body == nil
		})
		if done {
			completed = true
			t.Log("first complete DEBUG source", ordinal)
			break
		}
	}
	if !completed {
		t.Fatal("no actual DEBUG completion")
	}
}
