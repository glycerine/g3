package numeral

// Source: src/util/mpz.h:156-162 ensure_tmp_capacity. When the release of
// the old m_tmp fails (a native accounting failure after the physical
// release), z3 would be left with a dangling m_tmp. g3 keeps the cell with
// its release pending, and the next ensure_tmp finishes that release and
// allocates fresh scratch, so later arithmetic on the owner works.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestIntegerEnsureTmpResumesStoppedScratchRelease(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Cells: arena.Config{Budget: b, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}})
	serviceMust(t, e)
	z := m.integer
	// A wide scratch whose release reaches the native runtime.
	serviceMust(t, z.EnsureTmp(0, 2000))
	old := z.Tmp[0]
	contextTLSNearMinimum(t, r)
	lane, e := r.Allocate(8)
	serviceMust(t, e)
	delta := int64(-99950) - r.State().LocalSize
	if e = r.Reallocate(&lane, uint64(16)+uint64(delta)-8); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	if e = z.EnsureTmp(0, 5000); !errors.Is(e, arena.ErrBounds) || z.Tmp[0] != old {
		t.Fatal("scratch replacement did not stop at its release", e, z.Tmp[0], old)
	}
	// Restore the accounting; the next ensure_tmp must repair the scratch.
	recovery, e := r.Allocate(200000)
	serviceMust(t, e)
	if e = z.EnsureTmp(0, 5000); e != nil || z.Tmp[0] == 0 || z.Tmp[0] == old && z.Cell(old).Released {
		t.Fatal("stopped scratch release was not resumed", e, z.Tmp[0])
	}
	digits := make([]uint32, 3000)
	digits[2999] = 1
	var a, c Int
	serviceMust(t, z.SetLimbs(&a, digits))
	serviceMust(t, z.Add(&a, &a, &c))
	if bits, e := z.Log2(&c); e != nil || bits != 2999*32+1 {
		t.Fatal("arithmetic after the repaired scratch", bits, e)
	}
	serviceMust(t, z.Del(&a))
	serviceMust(t, z.Del(&c))
	contextTLSFinish(t, r, m.Close)
	for _, block := range []*umemory.Block{&lane, &recovery} {
		contextTLSFinish(t, r, func() error {
			if *block == (umemory.Block{}) {
				return nil
			}
			return r.Deallocate(block)
		})
	}
	contextTLSFinish(t, r, r.Close)
	if b.Used() != 0 {
		t.Fatal("refunds")
	}
}
