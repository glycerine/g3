package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1213-1240;
// src/util/mpzzp.h:46-56; src/util/mpz.cpp:213-242.

import (
	"fmt"
	"testing"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestUPolynomialRecombinationTrialExactOldLeafPreservesForeignReplacement(t *testing.T) {
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		found := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			q, s := mignotteDirectFixture(t)
			m := s.manager
			var prime, foreign, large unumeral.Int
			serviceMust(t, q.Integers().SetLimbs(&prime, []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0x7fffffff}))
			words := make([]uint32, 25000)
			words[24999] = 1
			serviceMust(t, q.Integers().SetLimbs(&large, words))
			defer func() {
				serviceMust(t, q.Integers().CleanupRawNumeral(&foreign))
				serviceMust(t, q.Integers().Del(&prime))
				serviceMust(t, q.Integers().Del(&large))
			}()
			_, e := s.newSquareFreeScope()
			serviceMust(t, e)
			fs, e := m.NewFactors()
			serviceMust(t, e)
			defer func() { serviceMust(t, fs.Close()) }()
			h, e := s.beginRecombinationTrial(fs)
			serviceMust(t, e)
			r := h.receipt
			q.Budget().FailAfter(ordinal / 16)
			body := m.modular.WithAllocationContext(integerAllocationContextRef((numericIntegerModularManager_trialAllocationContext(m.modular, h))).utility(m.modular.Integers())).Copy(&h.tmp, &prime)
			h.executing = false
			primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &foreign, &large)
			old := h.numeric.Cell
			if old.Object != (umemory.ObjectRef{}) {
				if q.Integers().PendingCell != old || !numericPhysicalReservationRefusal(body) {
					t.Fatal("actual direct pending leaf absent", ordinal, body, old, q.Integers().PendingCell)
				}
				_ = h.finish(&r, body)
				serviceMust(t, h.own(&r, false))
				if h.numeric.Cell != old || q.Integers().PendingCell != old {
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
					primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &foreign, &large)
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
						primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{fs}, &prime, &foreign, &large)
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
