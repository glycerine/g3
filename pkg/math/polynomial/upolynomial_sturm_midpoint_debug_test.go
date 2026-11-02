//go:build g3debug

package polynomial

import (
	"errors"
	"strconv"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestUPolynomialSturmMidpointDebugFailureReleasesBinaryChildren(t *testing.T) {
	reached := false
	for fault := int64(0); fault < 64 && !reached; fault++ {
		if !t.Run(strconv.FormatInt(fault, 10), func(t *testing.T) {
			m, q, budget := upolySourceOwner(t)
			b := upolyIsolationBinaryOwner(t, q, budget)
			r, l, u := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
			p := upolyIsolationProduct(t, m, [][]int32{{-1, 4}, {-3, 4}})
			t.Cleanup(func() {
				budget.FailAfter(-1)
				serviceMust(t, m.CleanupSturmIsolation())
			})
			budget.FailAfter(fault)
			body := m.SturmIsolateRootsWithBounds(mustUPolyView(t, m, p), 128, 128, b, r, l, u)
			if !numericStorageReservationRefusal(body) {
				t.Fatal("expected a real allocation refusal", fault, body)
			}
			stats, e := b.Stats()
			serviceMust(t, e)
			if stats.HeaderBytes == 0 {
				t.Fatal("scratch inspection lost the live manager")
			}
			upolyHenselAuditPhysical(t, []*UPolynomialManager{m}, nil, upolyPhysicalExtraOwners{BinaryManagers: []*unumeral.BinaryRationalManager{b}, BinaryVectors: []*BinaryRationalVector{r, l, u}})
			call := &m.sturmIsolation.binaryArithmetic
			if call.Pending() {
				reached = true
				copied := *call
				if e := copied.Cleanup(); !errors.Is(e, unumeral.ErrBinaryOwner) {
					t.Fatal("copied receipt authorized child cleanup", e)
				}
			}
			sourceStage, sourceCause := m.sturmIsolation.sourceStage, call.SourceCause()
			budget.FailAfter(-1)
			serviceMust(t, m.CleanupSturmIsolation())
			if call.Pending() || m.sturmIsolation.active {
				t.Fatal("owning cleanup left midpoint debug children")
			}
			if m.sturmIsolation.sourceStage != sourceStage || call.SourceCause() != sourceCause {
				t.Fatal("owning cleanup rewrote the source destruction history")
			}
			serviceMust(t, r.Reset())
			serviceMust(t, l.Reset())
			serviceMust(t, u.Reset())
		}) {
			return
		}
	}
	if !reached {
		t.Fatal("did not reach a failed midpoint diagnostic child")
	}
}
