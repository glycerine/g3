package numeral

// Source: src/util/mpf.cpp:1255-1270.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryFloatRenormalizeEveryColdReservation(t *testing.T) {
	for _, sb := range []uint32{2, 24, 80, 120} {
		for _, up := range []bool{false, true} {
			complete := false
			for ordinal := int64(0); ordinal < 256; ordinal++ {
				m, r, b := mpfNumeralHelpersFixture(t)
				z := m.integer
				sig := NewInt(1)
				if !up {
					serviceMust(t, z.Mul2k(&sig, sb+79, &sig))
					serviceMust(t, z.Inc(&sig))
				}
				exp := int64(-7)
				b.FailAfter(ordinal)
				body := m.Renormalize(8, sb, &exp, &sig)
				integerPhysicalAudit(t, z, &sig)
				if b.Used() != m.OwnedStorageBytes()+r.Stats().TotalBytes {
					t.Fatal("immediate renormalize graph", sb, up, ordinal, body, b.Used(), m.OwnedStorageBytes(), r.Stats())
				}
				if body != nil && !numericPhysicalReservationRefusal(body) {
					t.Fatal("unexpected cold failure", sb, up, ordinal, body)
				}
				if m.renormalize.active {
					if e := m.Renormalize(8, sb, &exp, &sig); !errors.Is(e, ErrNumericPending) {
						t.Fatal("stopped scope replay", e)
					}
				}
				b.FailAfter(-1)
				serviceMust(t, m.CleanupRenormalize())
				serviceMust(t, z.CleanupRawNumeral(&sig))
				if source := m.SourceFinalize(); source != nil {
					if !errors.Is(source, arena.ErrInvalidSlot) || !z.sourceGuard || z.SourceFinalizeStage < 2 || z.SourceFinalizeStage > 5 {
						t.Fatal("unexpected source stop", source, z.SourceFinalizeStage)
					}
					index := int(z.SourceFinalizeStage-2) / 2
					ref := z.Tmp[index]
					if z.SourceFinalizeStage%2 == 1 {
						ref = z.Arg[index]
					}
					if ref != 0 {
						t.Fatal("unproved source scratch hole", ref)
					}
				}
				serviceMust(t, m.Close())
				serviceMust(t, r.Close())
				if b.Used() != 0 {
					t.Fatal("renormalize refund", sb, up, ordinal, b.Used())
				}
				if body == nil {
					complete = true
					break
				}
			}
			if !complete {
				t.Fatal("renormalize reservation horizon", sb, up)
			}
		}
	}
}

func TestArbitraryFloatRenormalizeSignedExponentPublication(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	for _, up := range []bool{false, true} {
		sig, exp := NewInt(4), int64(1<<63-1)
		want := NewInt(2)
		if up {
			sig = NewInt(1)
			exp = -1 << 63
		}
		original := exp
		e := m.Renormalize(8, 2, &exp, &sig)
		if !errors.Is(e, arena.ErrBounds) || exp != original || sig != want {
			t.Fatal("reached undefined exponent after source shift", up, e, exp, sig)
		}
		serviceMust(t, m.CleanupRenormalize())
	}
	sig, exp := NewInt(0), int64(17)
	if e := m.renormalizeWithContext(8, 2, &exp, &sig, numeralContextRef(m.renormalize.context())); !errors.Is(e, ErrNumericOwner) {
		t.Fatal("ended scope acquired inline authority", e)
	}
	if sig != NewInt(0) || exp != 17 {
		t.Fatal("ended scope changed fields")
	}
}
