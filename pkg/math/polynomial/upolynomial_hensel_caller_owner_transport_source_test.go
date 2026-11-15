package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:583-674;
// src/math/polynomial/upolynomial_factorization_int.h:42-52;
// src/util/memory_manager.cpp:274-285.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

// ErrorTransport is an actual supported Go callback profile; this does not
// claim a native callback at the C++ allocation exception.
func TestUPolynomialHenselQuadraticTransportCannotRetireRelatedCaller(t *testing.T) {
	for _, mode := range []string{"service", "upm", "source"} {
		t.Run(mode, func(t *testing.T) {
			b := arena.NewBudget(1 << 26)
			var s *UPolynomialFactorizationService
			var u *UPolynomialManager
			var target *UPolynomialManager
			var foreign unumeral.Int
			armed := false
			calls := 0
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true, Transport: func(f umemory.Failure) error {
				if armed {
					calls++
					// This actual nil-context Copy publishes into an independently caller-owned
					// BIG destination with sufficient existing capacity; it allocates no storage.
					serviceMust(t, u.config.Integer.Copy(&foreign, &u.config.Integer.Two64))
					before := rtMignotteOwnerSnapshot(target, s)
					var stop error
					if mode == "service" {
						stop = s.CleanupRetained()
					} else if mode == "source" {
						stop = target.SourceFinalize()
					} else {
						stop = target.Close()
					}
					if stop != ErrUPolynomialCleanupPending || rtMignotteOwnerSnapshot(target, s) != before {
						t.Errorf("executing owner changed in transport: mode=%s error=%v before=%+v after=%+v", mode, stop, before, rtMignotteOwnerSnapshot(target, s))
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
			u, e = NewUPolynomialManager(UPolynomialConfig{Integer: q.Integers(), Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }})
			serviceMust(t, e)
			primes, e := primegen.NewGenerator(b)
			serviceMust(t, e)
			s, e = NewUPolynomialFactorizationService(u, UPolynomialFactorizationConfig{Primes: primes})
			serviceMust(t, e)
			defer func() {
				armed = false
				b.FailAfter(-1)
				serviceMust(t, rt.SetMaxSize(0))
				serviceMust(t, s.Close())
				serviceMust(t, primes.Close())
				if !u.closed {
					serviceMust(t, u.Close())
				}
				serviceMust(t, q.CloseChecked())
				serviceMust(t, rt.Close())
				if b.Used() != 0 {
					t.Error("named refund", b.Used())
				}
			}()
			var big unumeral.Int
			serviceMust(t, q.Integers().SetLimbs(&big, []uint32{1, 0, 1}))
			words := make([]uint32, 80)
			words[79] = 1
			serviceMust(t, q.Integers().SetLimbs(&foreign, words))
			defer func() { serviceMust(t, q.Integers().Del(&foreign)) }()
			defer func() { serviceMust(t, q.Integers().Del(&big)) }()
			parent, e := s.newSquareFreeScope()
			serviceMust(t, e)
			serviceMust(t, s.beginHenselCaller(parent))
			caller := &parent.hensel
			caller.related, e = NewRelatedUPolynomialManager(u)
			serviceMust(t, e)
			target = caller.related
			caller.enterCall()
			pad := int64(99996) - rt.State().LocalSize - 12
			if pad <= 0 {
				t.Fatal("real threshold", rt.State())
			}
			held, e := rt.Allocate(uint64(pad))
			serviceMust(t, e)
			defer func() { serviceMust(t, rt.Deallocate(&held)) }()
			serviceMust(t, rt.SetMaxSize(1))
			armed = true
			body := target.modular.WithAllocationContext(integerAllocationContextRef((numericIntegerModularManager_henselCallerAllocationContext(target.modular, caller))).utility(target.modular.Integers())).SetZP(&big)
			armed = false
			caller.executing = false
			caller.leaveCall()
			if !numericPhysicalReservationRefusal(body) || calls != 1 {
				t.Fatal("actual allocation transport absent", body, calls, rt.State())
			}
			primeTrialPhysicalAuditExternal(t, q, s, []umemory.Block{held}, &big, &foreign)
			if !polynomialNegNumericPending(&caller.numeric) {
				t.Fatal("real returning allocation lost exact pending receipt")
			}
			_ = caller.finish()
			serviceMust(t, caller.own(false))
			serviceMust(t, s.CleanupRetained())
			primeTrialPhysicalAuditExternal(t, q, s, []umemory.Block{held}, &big, &foreign)
			same, e := q.Integers().Equal(&foreign, &q.Integers().Two64)
			serviceMust(t, e)
			if !same {
				t.Fatal("owning consumed callback foreign numeral")
			}
			serviceMust(t, rt.SetMaxSize(0))
		})
	}
}
