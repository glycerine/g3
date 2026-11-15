package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1058-1094.

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

// Actual first loop checkpoint follows gcd_tmp construction. It is not a
// fabricated allocation callback between source actions or an owning reset.
func TestUPolynomialFactorPrimePreparationActualCheckpointOwnerRefusal(t *testing.T) {
	for _, mode := range []string{"own", "source", "root-close", "root-source", "root-mode", "related-close", "related-source", "related-mode"} {
		t.Run(mode, func(t *testing.T) { primePreparationCheckpointOwnerRefusal(t, mode) })
	}
}
func primePreparationCheckpointOwnerRefusal(t *testing.T, mode string) {
	b := arena.NewBudget(1 << 26)
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: serviceConfig(b).Integers})
	serviceMust(t, e)
	var s *UPolynomialFactorizationService
	calls := 0
	m, e := NewUPolynomialManager(UPolynomialConfig{Integer: q.Integers(), Checkpoint: func() bool {
		if s != nil && s.scope != nil && s.scope.constructed == 7 {
			calls++
			c := s.scope
			before := *c
			var stop error
			switch mode {
			case "own":
				stop = s.CleanupRetained()
			case "source":
				stop = s.finishSquareFreeScopeSource()
			case "root-close":
				stop = s.manager.Close()
			case "root-source":
				stop = s.manager.SourceFinalize()
			case "root-mode":
				stop = s.manager.SetZ()
			case "related-close":
				stop = c.zp.Close()
			case "related-source":
				stop = c.zp.SourceFinalize()
			case "related-mode":
				stop = c.zp.SetZ()
			}
			if stop != ErrUPolynomialCleanupPending || s.scope != c || !reflect.DeepEqual(*c, before) {
				t.Errorf("actual prime preparation checkpoint retired live caller: %v", stop)
			}
			return false
		}
		return true
	}, Cooperation: func() error { return nil }})
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(b)
	serviceMust(t, e)
	s, e = NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
	serviceMust(t, e)
	src, e := m.NewFactors()
	serviceMust(t, e)
	out, e := m.NewFactors()
	serviceMust(t, e)
	defer func() {
		b.FailAfter(-1)
		serviceMust(t, s.CleanupRetained())
		serviceMust(t, src.Close())
		serviceMust(t, out.Close())
		serviceMust(t, s.Close())
		serviceMust(t, primes.Close())
		serviceMust(t, m.Close())
		serviceMust(t, q.CloseChecked())
		if b.Used() != 0 {
			t.Error("real owner refunds", b.Used())
		}
	}()
	var f upolyVectorState
	serviceMust(t, m.set(&f, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), unumeral.NewInt(0), unumeral.NewInt(1)}))
	serviceMust(t, src.PushCopy(m.stateView(&f), 1))
	serviceMust(t, m.reset(&f))
	m.freeBacking(&f)
	original, e := src.factor(0)
	serviceMust(t, e)
	_, body := s.FactorSquareFree(m.stateView(original), out, 1, DefaultUPolynomialFactorParams())
	if calls != 1 || body == nil {
		t.Fatal("actual first checkpoint not reached", calls, body)
	}
	primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{src, out})
}

// Arm the real reservation failure only at the first source checkpoint,
// after every enclosing constructor. The first BIG GCD copy is then reached.
func TestUPolynomialFactorPrimePreparationActualGCDLeafOwning(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: serviceConfig(b).Integers})
	serviceMust(t, e)
	var s *UPolynomialFactorizationService
	armed := false
	m, e := NewUPolynomialManager(UPolynomialConfig{Integer: q.Integers(), Checkpoint: func() bool {
		if s != nil && s.scope != nil && s.scope.constructed == 7 && !armed {
			armed = true
			b.FailAfter(0)
		}
		return true
	}, Cooperation: func() error { return nil }})
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(b)
	serviceMust(t, e)
	s, e = NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{Primes: primes})
	serviceMust(t, e)
	src, e := m.NewFactors()
	serviceMust(t, e)
	out, e := m.NewFactors()
	serviceMust(t, e)
	var large unumeral.Int
	defer func() {
		b.FailAfter(-1)
		serviceMust(t, s.CleanupRetained())
		serviceMust(t, q.Integers().CleanupGCDRaw())
		serviceMust(t, src.Close())
		serviceMust(t, out.Close())
		serviceMust(t, q.Integers().CleanupRawNumeral(&large))
		serviceMust(t, s.Close())
		serviceMust(t, primes.Close())
		serviceMust(t, m.Close())
		serviceMust(t, q.CloseChecked())
		if b.Used() != 0 {
			t.Error("actual refunds", b.Used())
		}
	}()
	serviceMust(t, q.Integers().SetLimbs(&large, []uint32{0, 0, 2}))
	var f upolyVectorState
	serviceMust(t, m.set(&f, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), unumeral.NewInt(0), large}))
	serviceMust(t, src.PushCopy(m.stateView(&f), 1))
	serviceMust(t, m.reset(&f))
	m.freeBacking(&f)
	original, e := src.factor(0)
	serviceMust(t, e)
	_, body := s.FactorSquareFree(m.stateView(original), out, 1, DefaultUPolynomialFactorParams())
	if !armed || !numericPhysicalReservationRefusal(body) {
		t.Fatal("actual reached GCD Copy refusal", armed, body)
	}
	primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{src, out}, &large)
	// Explicit own acknowledgment of the actual caller must drain its captured
	// failed GCD child; source-only enclosing completion cannot auto-abort RAW5.
	b.FailAfter(-1)
	serviceMust(t, s.CleanupRetained())
	if q.Integers().GcdRaw.Active {
		t.Error("actual caller lost failed GCD RAW5 owning receipt")
	}
}

// Exact bounded source driver: the accepted preceding f_pp preparation is
// complete before arming faults; the selected constructors/prime loop and
// automatic enclosing source epilogue are the actual production engines.
func TestUPolynomialFactorPrimePreparationEveryActualPrefix(t *testing.T) {
	for _, mode := range []string{"early", "eligible", "rejected"} {
		complete := false
		for ordinal := int64(0); ordinal < 256; ordinal++ {
			done := false
			t.Run(mode+"/"+fmt.Sprint(ordinal), func(t *testing.T) {
				q, s := preparationPhysicalFixture(t)
				m := s.manager
				src, e := m.NewFactors()
				serviceMust(t, e)
				out, e := m.NewFactors()
				serviceMust(t, e)
				var large unumeral.Int
				serviceMust(t, q.Integers().SetLimbs(&large, []uint32{0, 0, 2}))
				if mode == "rejected" {
					three := unumeral.NewInt(3)
					serviceMust(t, q.Integers().Mul(&large, &three, &large))
				}
				defer func() {
					q.Budget().FailAfter(-1)
					serviceMust(t, s.CleanupRetained())
					serviceMust(t, src.Close())
					serviceMust(t, out.Close())
					serviceMust(t, q.Integers().CleanupRawNumeral(&large))
				}()
				var f upolyVectorState
				serviceMust(t, m.set(&f, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), unumeral.NewInt(0), large}))
				serviceMust(t, src.PushCopy(m.stateView(&f), 1))
				serviceMust(t, m.reset(&f))
				m.freeBacking(&f)
				original, e := src.factor(0)
				serviceMust(t, e)
				c, e := s.newSquareFreeScope()
				serviceMust(t, e)
				c.constructed = 1
				serviceMust(t, s.prepareSquareFree(m.stateView(original), out, 1))
				q.Budget().FailAfter(ordinal)
				body := s.preparePrimeSearch(out)
				eligible := false
				if body == nil {
					params := DefaultUPolynomialFactorParams()
					params.MaxPrime = 3
					if mode == "early" {
						params.MaxPrime = 2
					}
					eligible, body = s.nextPreparedPrime(out, 1, params)
				}
				if body == nil {
					if eligible != (mode == "eligible") {
						t.Fatal("literal gcd/early route", mode, eligible)
					}
					one := unumeral.NewInt(1)
					three := unumeral.NewInt(3)
					if mode == "eligible" {
						same, e := q.Integers().Equal(&c.gcdTmp, &one)
						serviceMust(t, e)
						if !same {
							t.Fatal("independent gcd expected1")
						}
					}
					if mode == "rejected" {
						same, e := q.Integers().Equal(&c.gcdTmp, &three)
						serviceMust(t, e)
						if !same || c.iterator.Index != 3 {
							t.Fatal("literal continue must reach next prime", c.iterator.Index)
						}
					}
					if mode != "eligible" {
						if out.count != 1 {
							t.Fatal("partial early publication", out.count)
						}
						st, e := out.factor(0)
						serviceMust(t, e)
						same, e := m.EqualViews(m.stateView(st), m.stateView(original))
						serviceMust(t, e)
						if !same {
							t.Fatal("actual early factor differs")
						}
					}
					done = true
				} else if !numericPhysicalReservationRefusal(body) {
					t.Fatal("actual source refusal", body)
				}
				body = s.finishSquareFreeScopeError(body)
				primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{src, out}, &large)
				h := &c.primePreparation
				r := h.receipt
				firstBody, firstSource := h.bodyCause, h.sourceCause
				// At exhausted budget no older source NUM exists outside this caller.
				// False acknowledgment cannot own abort a failed RAW allocation body.
				if h.active {
					before := h.numeric
					stop := h.own(&r, false)
					if stop != nil && !numericPhysicalReservationRefusal(stop) {
						t.Fatal("actual completed child acknowledgment", stop)
					}
					if (h.numeric).NumericAllocationRoots != (before).NumericAllocationRoots {
						t.Fatal("false phase retired direct unjoined roots")
					}
					primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{src, out}, &large)
				}
				q.Budget().FailAfter(-1)
				serviceMust(t, s.CleanupRetained())
				if s.scope != nil || h.active || h.bodyCause != firstBody || h.sourceCause != firstSource {
					t.Fatal("exact owning/source history", body)
				}
				primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{src, out}, &large)
			})
			if done {
				complete = true
				t.Log("first actual complete", mode, ordinal)
				break
			}
		}
		if !complete {
			t.Fatal("no selected source completion", mode)
		}
	}
}

func TestUPolynomialFactorPrimePreparationExactContextHistoryGuards(t *testing.T) {
	q, s := preparationPhysicalFixture(t)
	m := s.manager
	fs, e := m.NewFactors()
	serviceMust(t, e)
	defer func() { serviceMust(t, s.CleanupRetained()); serviceMust(t, fs.Close()) }()
	c, e := s.newSquareFreeScope()
	serviceMust(t, e)
	c.constructed = 1
	serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(1)}))
	h, e := s.beginPrimePreparation(fs)
	serviceMust(t, e)
	r := h.receipt
	if (integerAllocationContextRef(numericIntegerModularManager_primePreparationAllocationContext(m.modular, h)).roots(q.Integers())) != nil {
		t.Fatal("adapter metadata before p constructor owns no NUM")
	}
	c.constructed = 2
	context := numericIntegerModularManager_primePreparationAllocationContext(m.modular, h)
	if (context.roots(q.Integers())) != (&h.numeric) {
		t.Fatal("actual embedded frame not bound")
	}
	before := *h
	if h.own(&r, true) != ErrUPolynomialCleanupPending || h.finish(&r, nil) != ErrUPolynomialCleanupPending || !reflect.DeepEqual(*h, before) {
		t.Fatal("executing rejection mutated exact history")
	}
	copyFrame := *h
	copied := context
	copied.primePreparation = &copyFrame
	if (copied.roots(q.Integers())) != nil {
		t.Fatal("copied frame acquired roots")
	}
	copiedReceipt := r
	copiedReceipt.frame = &copyFrame
	if h.own(&copiedReceipt, true) != ErrUPolynomialOwner || !reflect.DeepEqual(*h, before) {
		t.Fatal("copied receipt cleanup")
	}
	m.callbackDepth++
	if (context.roots(q.Integers())) != nil {
		t.Fatal("callback frame acquired roots")
	}
	m.callbackDepth--
	other, e := NewRelatedUPolynomialManager(m)
	serviceMust(t, e)
	if numericIntegerModularManager_primePreparationAllocationContext(other.modular, h) != (integerAllocationInvocationContext{}) {
		t.Fatal("foreign related modular receiver acquired source roots")
	}
	serviceMust(t, other.Close())
	h.executing = false
	serviceMust(t, h.finish(&r, nil))
	serviceMust(t, h.own(&r, false))
	serviceMust(t, h.own(&r, true))
	fresh, e := s.beginPrimePreparation(fs)
	serviceMust(t, e)
	if fresh != h || fresh.invocation != r.invocation+1 {
		t.Fatal("actual next nonwrap invocation")
	}
	now := *fresh
	if (context.roots(q.Integers())) != nil || h.own(&r, true) != ErrUPolynomialOwner || !reflect.DeepEqual(*fresh, now) {
		t.Fatal("stale captured context/receipt retargeted fresh frame")
	}
	fresh.executing = false
	nr := fresh.receipt
	serviceMust(t, fresh.finish(&nr, nil))
	primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs})
}

func TestUPolynomialFactorPrimePreparationOldLeafPreservesForeignReplacement(t *testing.T) {
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		found := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			q, s := preparationPhysicalFixture(t)
			m := s.manager
			var prime, foreign, large, value, heldAfter unumeral.Int
			oldWords := make([]uint32, 25000)
			oldWords[0] = 1
			oldWords[24999] = 1
			serviceMust(t, q.Integers().SetLimbs(&prime, oldWords))
			words := make([]uint32, 25000)
			words[24999] = 1
			serviceMust(t, q.Integers().SetLimbs(&large, words))
			defer func() {
				serviceMust(t, q.Integers().CleanupRawNumeral(&heldAfter))
				serviceMust(t, q.Integers().CleanupRawNumeral(&foreign))
				serviceMust(t, q.Integers().CleanupRawNumeral(&value))
				serviceMust(t, q.Integers().Del(&prime))
				serviceMust(t, q.Integers().Del(&large))
			}()
			fs, e := m.NewFactors()
			serviceMust(t, e)
			defer func() { serviceMust(t, fs.Close()) }()
			var original upolyVectorState
			serviceMust(t, m.set(&original, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), unumeral.NewInt(0), prime}))
			serviceMust(t, fs.PushCopy(m.stateView(&original), 1))
			serviceMust(t, m.reset(&original))
			m.freeBacking(&original)
			state, e := fs.factor(0)
			serviceMust(t, e)
			// Real held NUM consumes the slot actually freed by the source vector.
			// This is a cold owner-primitive profile, not a fabricated reservation.

			out, e := m.NewFactors()
			serviceMust(t, e)
			defer func() { serviceMust(t, out.Close()) }()
			c, e := s.newSquareFreeScope()
			serviceMust(t, e)
			c.constructed = 1
			serviceMust(t, s.prepareSquareFree(m.stateView(state), out, 1))
			serviceMust(t, s.preparePrimeSearch(out))
			serviceMust(t, q.Integers().Copy(&value, &prime))
			q.Budget().FailAfter(ordinal / 16)
			params := DefaultUPolynomialFactorParams()
			params.MaxPrime = 3
			_, body := s.nextPreparedPrime(out, 1, params)
			h := &c.primePreparation
			r := h.receipt

			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs, out}, &prime, &foreign, &large, &value, &heldAfter)
			owned := &h.numeric.NumericAllocationRoots
			if !unumeral.NumericAllocationPending(owned) && q.Integers().MatchesRawInvocation(&h.gcdRaw) {
				owned = &h.gcdRaw.Frame.Numeric
			}
			old := owned.Cell
			oldRoots := *owned
			if ordinal%16 == 0 {
				t.Logf("actual GCD prefix=%d body=%v rawActive=%v rawCount=%d captured=%v numeric=%+v pending=%+v", ordinal/16, body, q.Integers().GcdRaw.Active, q.Integers().GcdRaw.Count, h.gcdRaw.Frame == &q.Integers().GcdRaw, *owned, q.Integers().PendingCell)
			}
			if unumeral.NumericAllocationPending(owned) {
				if (old.Object != (umemory.ObjectRef{}) && q.Integers().PendingCell != old) || !numericPhysicalReservationRefusal(body) {
					t.Fatal("actual direct pending leaf absent", ordinal, body, old, q.Integers().PendingCell)
				}
				_ = h.finish(&r, body)
				serviceMust(t, h.own(&r, false))
				if *owned != oldRoots || (old.Object != (umemory.ObjectRef{}) && q.Integers().PendingCell != old) {
					t.Fatal("false phase retired unjoined raw leaf")
				}
				_ = s.finishSquareFreeScopeError(body)
				primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs, out}, &prime, &foreign, &large, &value, &heldAfter)
				if c.constructed != 0 {
					t.Fatal("older actual source fields did not finish before unjoined owning abort", c.constructed)
				}
				q.Budget().FailAfter(-1)
				serviceMust(t, q.Integers().CleanupPendingAllocation())
				// The completed source f_pp DT recycled its real BIG descriptor.
				// A genuine independent held NUM consumes that slot before the foreign Copy.
				serviceMust(t, q.Integers().Copy(&heldAfter, &prime))
				// Actual independent nil-context leaf starts after the old token is consumed.
				// Search its real reservation prefixes rather than minting a pending token.
				foreignFound := false
				for next := ordinal % 16; next < ordinal%16+1; next++ {
					q.Budget().FailAfter(next)
					failure := q.Integers().Copy(&foreign, &large)
					primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs, out}, &prime, &foreign, &large, &value, &heldAfter)
					t.Logf("foreignCopy prefix=%d error=%v pending=%+v", next, failure, q.Integers().PendingCell)
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
						primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs, out}, &prime, &foreign, &large, &value, &heldAfter)
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
				serviceMust(t, s.CleanupRetained())
			} else {
				q.Budget().FailAfter(-1)
				_ = h.finish(&r, body)
				serviceMust(t, h.own(&r, false))
				_ = s.finishSquareFreeScopeError(body)
				serviceMust(t, h.own(&r, true))
				serviceMust(t, s.CleanupRetained())
			}
		})
		if found {
			return
		}
	}
	t.Fatal("no actual direct old descriptor prefix")
}

// Reached lexical NUM release profile; signed-host TLS guard, not ordinary OOM.
func TestUPolynomialFactorPrimePreparationActualNumeralDestructorHistory(t *testing.T) {
	for _, source := range []bool{true} {
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
			serviceMust(t, s.preparePrimeSearch(fs))
			h := &parent.primePreparation
			r := h.receipt
			serviceMust(t, zq.Integers().Copy(&parent.gcdTmp, &zq.Integers().Two64))
			serviceMust(t, zq.Integers().EnsureOwnedCapacity(&parent.gcdTmp, 40000))
			original := parent.fpp
			contextTLSNearMinimum(t, rt)
			blocks := rt.Stats().LiveBlocks
			var first error
			if source {
				first = s.finishSquareFreeScopeSource()
			} else {
				first = h.own(&r, false)
			}
			if !errors.Is(first, arena.ErrBounds) || rt.Stats().LiveBlocks+1 != blocks {
				t.Fatal("actual gcd_tmp source NUM release", first)
			}
			if parent.fpp != original || parent.constructed != 7 {
				t.Fatal("younger gcd_tmp DT changed older unread suffix")
			}
			primeTrialPhysicalAuditOwners(t, zq, s, []*UPolynomialFactors{fs})
			if source {
				before := *h
				if _, refusal := s.beginPrimePreparation(fs); refusal != ErrUPolynomialCleanupPending || !reflect.DeepEqual(*h, before) {
					t.Fatal("new begin overwrote stopped source history", refusal)
				}
				again := s.finishSquareFreeScopeSource()
				if !errors.Is(again, h.sourceCause) || parent.fpp != original || !reflect.DeepEqual(*h, before) {
					t.Fatal("source retry replayed older suffix", again)
				}
			} else if h.ownCause != first || h.ownEnded {
				t.Fatal("first own stop")
			}
			serviceMust(t, h.own(&r, false))
			if parent.constructed != 7 || parent.fpp != original || h.sourceCause == nil {
				t.Fatal("false own changed older suffix/history")
			}
			recovery, e := rt.Allocate(400000)
			serviceMust(t, e)
			contextTLSFinish(t, rt, s.CleanupRetained)
			if source && !errors.Is(first, h.sourceCause) || !source && h.ownCause != first {
				t.Fatal("matched owning acknowledgment lost immutable history")
			}
			primeTrialPhysicalAuditAll(t, zq, s, []*UPolynomialFactors{fs}, []umemory.Block{recovery})
			var nextInput upolyVectorState
			serviceMust(t, m.set(&nextInput, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), unumeral.NewInt(2)}))
			lateParams := DefaultUPolynomialFactorParams()
			lateParams.MaxPrime = 2
			_, later := s.FactorSquareFree(m.stateView(&nextInput), fs, 1, lateParams)
			serviceMust(t, m.reset(&nextInput))
			m.freeBacking(&nextInput)
			serviceMust(t, later)
			if !errors.Is(first, h.sourceCause) || !errors.Is(h.finish(&r, nil), h.sourceCause) {
				t.Fatal("new actual caller erased old returned/history cause")
			}
			serviceMust(t, h.own(&r, false))
			serviceMust(t, h.own(&r, true))
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

// Actual related class reverse private-member DT, with older caller p unread.
func TestUPolynomialFactorPrimePreparationActualRelatedDestructorHistory(t *testing.T) {
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
			serviceMust(t, s.preparePrimeSearch(fs))
			h := &parent.primePreparation
			r := h.receipt
			serviceMust(t, parent.zp.SetZP(&zq.Integers().Two64))
			serviceMust(t, zq.Integers().EnsureOwnedCapacity(&(*parent.zp.modular.BorrowModulus()), 40000))
			original := parent.fpp
			contextTLSNearMinimum(t, rt)
			blocks := rt.Stats().LiveBlocks
			var first error
			if source {
				first = s.finishSquareFreeScopeSource()
			} else {
				first = s.CleanupRetained()
			}
			if !errors.Is(first, arena.ErrBounds) || rt.Stats().LiveBlocks+1 != blocks {
				t.Fatal("actual related class private NUM release", first)
			}
			if parent.fpp != original || parent.constructed != 3 {
				t.Fatal("younger gcd_tmp DT changed older unread suffix")
			}
			primeTrialPhysicalAuditOwners(t, zq, s, []*UPolynomialFactors{fs})
			if source {
				before := *h
				if _, refusal := s.beginPrimePreparation(fs); refusal != ErrUPolynomialCleanupPending || !reflect.DeepEqual(*h, before) {
					t.Fatal("new begin overwrote stopped source history", refusal)
				}
				again := s.finishSquareFreeScopeSource()
				if !errors.Is(again, h.sourceCause) || parent.fpp != original || !reflect.DeepEqual(*h, before) {
					t.Fatal("source retry replayed older suffix", again)
				}
			} else if h.ownCause != first || h.ownEnded {
				t.Fatal("first own stop")
			}
			serviceMust(t, h.own(&r, false))
			if parent.constructed != 3 || parent.fpp != original || (source && h.sourceCause == nil) || (!source && h.ownCause == nil) {
				t.Fatal("false own changed older suffix/history")
			}
			recovery, e := rt.Allocate(400000)
			serviceMust(t, e)
			contextTLSFinish(t, rt, s.CleanupRetained)
			if source && !errors.Is(first, h.sourceCause) || !source && h.ownCause != first {
				t.Fatal("matched owning acknowledgment lost immutable history")
			}
			primeTrialPhysicalAuditAll(t, zq, s, []*UPolynomialFactors{fs}, []umemory.Block{recovery})
			var nextInput upolyVectorState
			serviceMust(t, m.set(&nextInput, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), unumeral.NewInt(2)}))
			lateParams := DefaultUPolynomialFactorParams()
			lateParams.MaxPrime = 2
			_, later := s.FactorSquareFree(m.stateView(&nextInput), fs, 1, lateParams)
			serviceMust(t, m.reset(&nextInput))
			m.freeBacking(&nextInput)
			serviceMust(t, later)
			if source && (!errors.Is(first, h.sourceCause) || !errors.Is(h.finish(&r, nil), h.sourceCause)) || !source && h.ownCause != first {
				t.Fatal("new actual caller erased old returned/history cause")
			}
			serviceMust(t, h.own(&r, false))
			serviceMust(t, h.own(&r, true))
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
