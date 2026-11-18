package numeral

// A copy that must grow its destination first releases the destination's
// old cell (mpz_manager::big_set). If that release stops on a native
// accounting failure, the destination keeps the old cell with its release
// pending; once memory is available again the next copy or deletion of the
// destination must finish that release, not report a dead slot.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestRationalCopyGrowthResumesStoppedRelease(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Cells: arena.Config{Budget: b, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}})
	serviceMust(t, e)
	q := m.rational
	small, wide := make([]uint32, 2000), make([]uint32, 4000)
	small[1999], wide[3999] = 1, 1
	dst, src := NewRational(), NewRational()
	serviceMust(t, q.SetLimbs(&dst, small))
	serviceMust(t, q.SetLimbs(&src, wide))
	contextTLSNearMinimum(t, r)
	lane, e := r.Allocate(8)
	serviceMust(t, e)
	delta := int64(-99950) - r.State().LocalSize
	if e = r.Reallocate(&lane, uint64(16)+uint64(delta)-8); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	if e = q.Copy(&dst, &src); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("copy growth did not stop at its release", e)
	}
	recovery, e := r.Allocate(200000)
	serviceMust(t, e)
	if e = q.Copy(&dst, &src); e != nil {
		t.Fatal("copy after the stopped release", e)
	}
	if same, e := q.Equal(&dst, &src); e != nil || !same {
		t.Fatal("copied value", same, e)
	}
	serviceMust(t, q.Del(&dst))
	serviceMust(t, q.Del(&src))
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
