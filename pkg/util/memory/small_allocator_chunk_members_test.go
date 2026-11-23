package memory

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// A direct allocation can survive a reset. Some of its surrounding location
// slots may already have been recycled into other chunks by Consolidate.
func TestSmallAllocatorInterleavedChunkMembership(t *testing.T) {
	a, _, _ := smallObjectFixture(t, SmallAllocatorProfile{PointerBits: 64})
	direct := smallObjectAlloc(t, a, 512)
	smallObjectMust(t, a.Write(direct, 3, 0xad))
	var class24, class40 []ObjectRef
	for i := 0; i < 700; i++ {
		class24 = append(class24, smallObjectAlloc(t, a, 24))
		class40 = append(class40, smallObjectAlloc(t, a, 40))
	}
	for _, r := range class24[:340] {
		smallObjectMust(t, a.Deallocate(24, r))
	}
	chunks := a.Stats().Chunks
	smallObjectMust(t, a.Consolidate())
	// The full 24-byte chunk was freed. The interleaved class and the next
	// 24-byte chunk both remain owned and usable.
	if remaining := a.Stats().Chunks; remaining+1 != chunks {
		t.Fatalf("consolidation retained %d of %d chunks", remaining, chunks)
	}
	for _, r := range []ObjectRef{class24[340], class40[0], class40[699]} {
		smallObjectMust(t, a.Write(r, 0, 0xa1))
	}
	var after []ObjectRef
	for i := 0; i < 500; i++ {
		after = append(after, smallObjectAlloc(t, a, 48))
	}
	reused := class40[499]
	smallObjectMust(t, a.Deallocate(40, reused))
	r := smallObjectAlloc(t, a, 40)
	if r.slot != reused.slot || r.generation == reused.generation {
		t.Fatal("expected size-class reuse", r, reused)
	}
	class40 = append(class40, r)
	smallObjectMust(t, a.Reset())
	for _, group := range [][]ObjectRef{class24, class40, after} {
		for _, stale := range group {
			if _, _, e := a.RawLocation(stale); !errors.Is(e, ErrAllocatorOwner) {
				t.Fatal("surviving pooled handle", stale, e)
			}
		}
	}
	if v, e := a.Read(direct, 3); e != nil || v != 0xad {
		t.Fatal("direct allocation lost across reset", v, e)
	}
	fresh := smallObjectAlloc(t, a, 24)
	smallObjectMust(t, a.Write(fresh, 0, 0xbc))
	smallObjectMust(t, a.Deallocate(512, direct))
}

func BenchmarkSmallAllocatorChunkLifecycle(b *testing.B) {
	for _, count := range []int{128, 4096, 32768, 65536} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			budget := arena.NewBudget(1 << 29)
			rt, err := NewRuntime(RuntimeConfig{Budget: budget, SizeTBits: 64})
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := rt.Close(); err != nil {
					b.Fatal(err)
				}
			}()
			a, err := NewSmallAllocator(SmallAllocatorConfig{Runtime: rt, Profile: SmallAllocatorProfile{PointerBits: 64}})
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := a.Close(); err != nil {
					b.Fatal(err)
				}
			}()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < count; j++ {
					if _, err := a.Allocate(uint64(8 * ((j % 15) + 1))); err != nil {
						b.Fatal(err)
					}
				}
				if err := a.Reset(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(count), "objects/op")
		})
	}
}
