package numeral

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestBinaryToOwnedRationalRetainsStoppedCleanupSuffix(t *testing.T) {
	qm, budget, runtime := rawArithmeticGuardOwner(t)
	config := serviceConfig(budget)
	config.Integers.Memory = runtime
	config.Integers.Profile = umemory.SmallAllocatorProfile{PointerBits: 32, Debug: true}
	s, e := NewRationalService(config)
	serviceMust(t, e)
	m, e := NewBinaryRationalManager(qm.Z, BinaryRationalConfig{Diagnostics: s})
	serviceMust(t, e)
	t.Cleanup(func() {
		serviceMust(t, m.CloseChecked())
		serviceMust(t, s.Close())
	})
	scope := &m.toOwnedRationalScope
	*scope = binaryOwnedRationalScope{service: s, pending: true, cursor: 2}
	scope.r, e = s.NewValue()
	serviceMust(t, e)
	scope.twok, e = s.NewValue()
	serviceMust(t, e)
	scope.result, e = s.NewValue()
	serviceMust(t, e)
	serviceMust(t, s.Manager.Z.SetLimbs(&scope.twok.Raw.Num, []uint32{7, 0, 1}))
	ref := scope.twok.Raw.Num.Cell
	original := s.Manager.Z.Cell(ref)
	bad := original
	bad.Backing.Requested = uint64(math.MaxUint32) + 1
	s.Manager.Z.Store(ref, bad)
	if e := m.CleanupToOwnedRational(); !errors.Is(e, arena.ErrBounds) || !scope.pending || scope.cursor != 2 || scope.twok.Raw.Num.Cell != ref || scope.r.ownerID == 0 || scope.result.ownerID == 0 {
		t.Fatal("physical refusal lost conversion suffix", e, scope.cursor)
	}
	if e := m.open(); !errors.Is(e, ErrBinaryOwnedRationalPending) {
		t.Fatal("retained conversion allowed reentry", e)
	}
	stats, statsErr := m.Stats()
	if statsErr != nil || stats.HeaderBytes != m.HeaderBytes || stats.LiveScratchNumerators != 0 {
		t.Fatal("pending conversion prevented scratch inspection", stats, statsErr)
	}
	s.Manager.Z.Store(ref, original)
	serviceMust(t, m.CleanupToOwnedRational())
	if scope.pending || scope.service != nil || scope.r.ownerID != 0 || scope.twok.ownerID != 0 || scope.result.ownerID != 0 {
		t.Fatal("owning retry retained conversion values")
	}
}
