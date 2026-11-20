package arena

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func newSmallTest(t *testing.T, chunk uint32, budget *Budget) *SmallBytes {
	t.Helper()
	a, e := NewSmallBytes(SmallBytesConfig{Alignment: 8, ChunkSize: chunk, Descriptors: Config{PageSize: 2, Budget: budget}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	return a
}
func smallAlloc(t *testing.T, a *SmallBytes, n uint32) SmallBytesRef {
	t.Helper()
	r, e := a.Allocate(n)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func smallFree(t *testing.T, a *SmallBytes, n uint32, r SmallBytesRef) {
	t.Helper()
	if e := a.Deallocate(n, r); e != nil {
		t.Fatal(e)
	}
}
func smallLocation(t *testing.T, a *SmallBytes, r SmallBytesRef) [2]uint32 {
	t.Helper()
	d, e := a.descriptor(r)
	if e != nil {
		t.Fatal(e)
	}
	return [2]uint32{d.page, d.offset}
}

// Source src/test/small_object_allocator.cpp has traces, not ENSURE assertions.
// Its finite allocation/free script is preserved; added identity/count checks are independent.
func TestSmallBytesSourceExercise(t *testing.T) {
	a := newSmallTest(t, 80, nil)
	p1 := smallAlloc(t, a, 13)
	q1 := smallAlloc(t, a, 14)
	p2 := smallAlloc(t, a, 13)
	last := smallLocation(t, a, p2)
	smallFree(t, a, 13, p1)
	smallFree(t, a, 14, q1)
	smallFree(t, a, 13, p2)
	p3 := smallAlloc(t, a, 13)
	if smallLocation(t, a, p3) != last {
		t.Fatal("class free list must be LIFO")
	}
	smallFree(t, a, 13, p3)
	var rs [4]SmallBytesRef
	for i := range rs {
		rs[i] = smallAlloc(t, a, 1)
	}
	smallFree(t, a, 1, rs[0])
	smallFree(t, a, 1, rs[2])
	rs[0] = smallAlloc(t, a, 1)
	smallFree(t, a, 1, rs[3])
	rs[3] = smallAlloc(t, a, 1)
	rs[2] = smallAlloc(t, a, 1)
	for _, r := range rs {
		smallFree(t, a, 1, r)
	}
	s := a.Stats()
	if s.Live != 0 || s.RequestedBytes != 0 || s.FreeObjects != 7 || s.WastedBytes != 80 {
		t.Fatalf("source exercise counts %+v", s)
	}
}
func TestSmallBytesBoundaryConsolidation(t *testing.T) {
	// Source uses strict < for subsequent slots and floor(chunk/class) for reclamation.
	// Dividing class16 in chunk64 gives only3 allocations per chunk, but reclaim expects4.
	a := newSmallTest(t, 64, nil)
	rs := make([]SmallBytesRef, 4)
	var locations [4][2]uint32
	for i := range rs {
		rs[i] = smallAlloc(t, a, 13)
		locations[i] = smallLocation(t, a, rs[i])
	}
	if a.Stats().Chunks != 2 {
		t.Fatal("exact endpoint must cause new chunk")
	}
	for _, r := range rs {
		smallFree(t, a, 13, r)
	}
	if e := a.Consolidate(); e != nil {
		t.Fatal(e)
	}
	if a.Stats().Chunks != 2 || a.Stats().FreeObjects != 4 {
		t.Fatal("must retain source exact-divisor and partial-chunk behavior")
	}
	// Rebuilding the list must preserve every free location, including its
	// zero-successor tail, across both chunks.
	for i := len(locations) - 1; i >= 0; i-- {
		ref := smallAlloc(t, a, 13)
		if got := smallLocation(t, a, ref); got != locations[i] {
			t.Fatalf("recycled location %v, want %v", got, locations[i])
		}
	}
	if s := a.Stats(); s.FreeObjects != 0 || s.Chunks != 2 {
		t.Fatalf("free list exhausted with %+v", s)
	}
	b := newSmallTest(t, 70, nil)
	for i := 0; i < 5; i++ {
		rs = append(rs, smallAlloc(t, b, 13))
	}
	live := rs[len(rs)-1]
	b.Write(live, 0, 99)
	for _, r := range rs[4:8] {
		smallFree(t, b, 13, r)
	}
	if e := b.Consolidate(); e != nil {
		t.Fatal(e)
	}
	if b.Stats().Chunks != 1 || b.Stats().FreeObjects != 0 {
		t.Fatalf("full nondivisor chunk not reclaimed %+v", b.Stats())
	}
	if v, e := b.Read(live, 0); e != nil || v != 99 {
		t.Fatal("live content moved or lost")
	}
	old := smallLocation(t, b, live)
	b.Consolidate()
	if smallLocation(t, b, live) != old {
		t.Fatal("consolidation moved live allocation")
	}
}
func TestSmallBytesStateMachine(t *testing.T) {
	a := newSmallTest(t, 70, nil)
	rng := rand.New(rand.NewSource(81))
	model := map[SmallBytesRef][]byte{}
	dead := []SmallBytesRef{}
	for step := 0; step < 3000; step++ {
		var chosen SmallBytesRef
		for r := range model {
			if chosen.Slot == 0 || r.Slot < chosen.Slot {
				chosen = r
			}
		}
		switch rng.Intn(6) {
		case 0, 1:
			n := uint32(rng.Intn(30) + 1)
			r := smallAlloc(t, a, n)
			data := make([]byte, n)
			for i := range data {
				data[i] = byte(rng.Intn(256))
				if e := a.Write(r, uint32(i), data[i]); e != nil {
					t.Fatal(e)
				}
			}
			model[r] = data
		case 2:
			if chosen.Slot != 0 {
				smallFree(t, a, uint32(len(model[chosen])), chosen)
				delete(model, chosen)
				dead = append(dead, chosen)
			}
		case 3:
			if e := a.Consolidate(); e != nil {
				t.Fatal(e)
			}
		case 4:
			if step%41 == 0 {
				a.Reset()
				for r := range model {
					dead = append(dead, r)
				}
				model = map[SmallBytesRef][]byte{}
			}
		}
		var requested uint64
		occupied := map[[2]uint32]bool{}
		for r, data := range model {
			requested += uint64(len(data))
			if !a.IsLive(r) {
				t.Fatal("lost live ref")
			}
			for i, want := range data {
				got, e := a.Read(r, uint32(i))
				if e != nil || got != want {
					t.Fatalf("step%d lost value", step)
				}
				loc := smallLocation(t, a, r)
				loc[1] += uint32(i)
				if occupied[loc] {
					t.Fatal("overlap")
				}
				occupied[loc] = true
			}
		}
		for _, r := range dead {
			if a.IsLive(r) {
				t.Fatal("stale handle resurrected")
			}
		}
		s := a.Stats()
		if uint64(s.Live) != uint64(len(model)) || s.RequestedBytes != requested {
			t.Fatal("logical accounting")
		}
	}
}
func TestSmallBytesFailureCleanup(t *testing.T) {
	for failure := int64(0); failure < 10; failure++ {
		b := NewBudget(1 << 20)
		a, e := NewSmallBytes(SmallBytesConfig{ChunkSize: 70, Descriptors: Config{PageSize: 2, Budget: b}})
		if e != nil {
			t.Fatal(e)
		}
		b.FailAfter(failure)
		r, e := a.Allocate(13)
		if e != nil {
			if !errors.Is(e, ErrInjectedFailure) {
				t.Fatal(e)
			}
			if a.Stats().Live != 0 || a.Stats().RequestedBytes != 0 {
				t.Fatal("failed allocate published")
			}
		} else {
			a.Write(r, 0, 21)
			b.FailAfter(0)
			if e = a.Deallocate(13, r); e != nil {
				t.Fatal("warm free must not grow", e)
			}
		}
		b.FailAfter(-1)
		a.Close()
		if b.Used() != 0 {
			t.Fatalf("failure%d leaked%d", failure, b.Used())
		}
	}
}
func TestSmallBytesValidationAndLedger(t *testing.T) {
	a := newSmallTest(t, 70, nil)
	z, e := a.Allocate(0)
	if e != nil || z != (SmallBytesRef{}) {
		t.Fatal("zero allocation")
	}
	if e = a.Deallocate(0, SmallBytesRef{Slot: 999}); e != nil {
		t.Fatal("source zero free is no-op")
	}
	r := smallAlloc(t, a, 13)
	if e = a.Deallocate(14, r); !errors.Is(e, ErrSizeMismatch) {
		t.Fatal("requested size mismatch accepted")
	}
	if _, e = a.Read(r, 13); !errors.Is(e, ErrBounds) {
		t.Fatal("bounds")
	}
	a.Reset()
	if a.IsLive(r) {
		t.Fatal("reset resurrected")
	}
	s := smallAlloc(t, a, 13)
	if s == r {
		t.Fatal("generation ledger lost")
	}
	if e = a.Deallocate(13, r); !errors.Is(e, ErrInvalidSlot) {
		t.Fatal("stale free accepted")
	}
	if e := ValidateRecordType(reflect.TypeOf(smallByteBlock{})); e != nil {
		t.Fatal(e)
	}
	if e := ValidateRecordType(reflect.TypeOf(SmallBytesRef{})); e != nil {
		t.Fatal(e)
	}
	a.Close()
	a.Close()
	if _, e = a.Allocate(1); !errors.Is(e, ErrClosed) {
		t.Fatal("closed")
	}
}
func TestSmallBytesPlateauAndWarmAllocation(t *testing.T) {
	b := NewBudget(1 << 20)
	a := newSmallTest(t, 70, b)
	for round := 0; round < 4; round++ {
		rs := make([]SmallBytesRef, 4)
		for i := range rs {
			rs[i] = smallAlloc(t, a, 13)
		}
		for _, r := range rs {
			smallFree(t, a, 13, r)
		}
		a.Consolidate()
	}
	baseline := b.Used()
	for round := 0; round < 100; round++ {
		var rs [4]SmallBytesRef
		for i := range rs {
			rs[i] = smallAlloc(t, a, 13)
		}
		for _, r := range rs {
			smallFree(t, a, 13, r)
		}
		a.Consolidate()
		if b.Used() != baseline {
			t.Fatal("consolidation churn grew metadata")
		}
	}
	r := smallAlloc(t, a, 13)
	smallFree(t, a, 13, r)
	if got := testing.AllocsPerRun(1000, func() {
		r, e := a.Allocate(13)
		if e != nil {
			panic(e)
		}
		if e = a.Deallocate(13, r); e != nil {
			panic(e)
		}
	}); got != 0 {
		t.Fatalf("warm allocations %v", got)
	}
}

func TestSmallBytesDirectOwnerLifetime(t *testing.T) {
	b := NewBudget(1 << 20)
	a := newSmallTest(t, 70, b)
	small := smallAlloc(t, a, 13)
	large := smallAlloc(t, a, 248)
	larger := smallAlloc(t, a, 1000)
	for i := uint32(0); i < 1000; i++ {
		if e := a.Write(larger, i, byte(i)); e != nil {
			t.Fatal(e)
		}
	}
	if a.Stats().RequestedBytes != 1261 || a.Stats().DirectObjects != 2 || a.Stats().DirectBytes != 1248 {
		t.Fatal("large requested accounting", a.Stats())
	}
	loc := smallLocation(t, a, larger)
	before := b.Used()
	smallFree(t, a, 248, large)
	if before-b.Used() != 248 {
		t.Fatal("direct payload not freed immediately")
	}
	if a.Stats().FreeObjects != 0 {
		t.Fatal("direct block joined pooled free list")
	}
	smallFree(t, a, 13, small)
	if e := a.Consolidate(); e != nil {
		t.Fatal(e)
	}
	if smallLocation(t, a, larger) != loc {
		t.Fatal("direct consolidation moved")
	}
	for i := uint32(0); i < 1000; i++ {
		got, e := a.Read(larger, i)
		if e != nil || got != byte(i) {
			t.Fatal("direct content lost")
		}
	}
	// Explicit Go owner cleanup adaptation: source requires callers to separately
	// free live direct blocks; this tracked owner invalidates/releases them at reset.
	a.Reset()
	if a.IsLive(larger) || a.Stats().DirectObjects != 0 || a.Stats().RequestedBytes != 0 {
		t.Fatal("reset did not reclaim owned direct allocation")
	}
	next := smallAlloc(t, a, 1000)
	if next == larger {
		t.Fatal("direct generation resurrected")
	}
	a.Close()
	if b.Used() != 0 {
		t.Fatal("direct close leaked", b.Used())
	}
}
func TestSmallBytesDirectFailuresAndThresholds(t *testing.T) {
	for _, alignment := range []uint32{4, 8} {
		a, e := NewSmallBytes(SmallBytesConfig{Alignment: alignment})
		if e != nil {
			t.Fatal(e)
		}
		threshold := 256 - alignment
		p := smallAlloc(t, a, threshold-1)
		d := smallAlloc(t, a, threshold)
		if a.Stats().DirectObjects != 1 {
			t.Fatal("threshold adaptation")
		}
		smallFree(t, a, threshold-1, p)
		smallFree(t, a, threshold, d)
		a.Close()
	}
	for fail := int64(0); fail < 8; fail++ {
		b := NewBudget(1 << 20)
		a := newSmallTest(t, 70, b)
		old := smallAlloc(t, a, 300)
		a.Write(old, 0, 43)
		b.FailAfter(fail)
		r, e := a.Allocate(400)
		if e != nil {
			if !errors.Is(e, ErrInjectedFailure) {
				t.Fatal(e)
			}
			if a.Stats().Live != 1 || a.Stats().RequestedBytes != 300 {
				t.Fatal("failed direct publication")
			}
		} else {
			smallFree(t, a, 400, r)
		}
		if v, e := a.Read(old, 0); e != nil || v != 43 {
			t.Fatal("failed direct affected live content")
		}
		b.FailAfter(-1)
		a.Close()
		if b.Used() != 0 {
			t.Fatal("failed direct leaked")
		}
	}
	b := NewBudget(1 << 20)
	a := newSmallTest(t, 70, b)
	r := smallAlloc(t, a, 300)
	smallFree(t, a, 300, r)
	if _, e := a.Allocate(^uint32(0)); !errors.Is(e, ErrBudget) && !errors.Is(e, ErrConfig) {
		t.Fatal("oversize should fail predictably", e)
	}
}
func TestSmallBytesGenerationRetirementAndAccounting(t *testing.T) {
	b := NewBudget(1 << 20)
	a := newSmallTest(t, 70, b)
	r := smallAlloc(t, a, 13)
	a.descriptors.state(r.Slot).generation = ^Generation(0)
	r.Generation = ^Generation(0)
	smallFree(t, a, 13, r)
	if a.descriptors.Stats().Retired != 1 {
		t.Fatal("descriptor not retired")
	}
	next := smallAlloc(t, a, 13)
	if next.Slot == r.Slot {
		t.Fatal("retired identity reused")
	}
	s := a.Stats()
	if b.Used() != s.PayloadBytes+s.MetadataBytes+s.DirectoryBytes+s.OwnerBytes {
		t.Fatal("unaccounted retained backing", b.Used(), s)
	}
	a.Reset()
	later := smallAlloc(t, a, 13)
	if later.Slot == r.Slot {
		t.Fatal("reset lost retirement ledger")
	}
	a.Close()
	if b.Used() != 0 {
		t.Fatal("retired cleanup leaked")
	}
}

func TestSmallBytesDirectoryGrowthPeakBudget(t *testing.T) {
	b := NewBudget(1 << 20)
	a := newSmallTest(t, 70, b)
	old := smallAlloc(t, a, 13)
	a.Write(old, 0, 87)
	entry := uint64(reflect.TypeOf(smallByteChunk{}).Size())
	// Final storage would fit exactly, but publication must first hold old and new
	// directory backing simultaneously, as well as the new chunk/block arrays.
	extra := entry + 70 + uint64(70/24)*uint64(reflect.TypeOf(smallByteBlock{}).Size())
	b.limit = b.Used() + extra
	if _, e := a.Allocate(17); !errors.Is(e, ErrBudget) {
		t.Fatal("old/new directory peak uncharged", e)
	}
	if a.Stats().Live != 1 || a.Stats().RequestedBytes != 13 {
		t.Fatal("peak failure published allocation")
	}
	if v, e := a.Read(old, 0); e != nil || v != 87 {
		t.Fatal("peak failure changed live data")
	}
	b.limit += entry
	fresh := smallAlloc(t, a, 17)
	smallFree(t, a, 17, fresh)
	a.Close()
	if b.Used() != 0 {
		t.Fatal("peak failure leaked")
	}
}

func BenchmarkSmallBytesWarm(b *testing.B) {
	a, e := NewSmallBytes(SmallBytesConfig{})
	if e != nil {
		b.Fatal(e)
	}
	defer a.Close()
	r, e := a.Allocate(13)
	if e != nil {
		b.Fatal(e)
	}
	a.Deallocate(13, r)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, e := a.Allocate(13)
		if e != nil {
			b.Fatal(e)
		}
		if e = a.Deallocate(13, r); e != nil {
			b.Fatal(e)
		}
	}
}

func TestSmallBytesDirectOnlyDiagnostics(t *testing.T) {
	budget := NewBudget(1 << 20)
	a, e := NewSmallBytes(SmallBytesConfig{DirectOnly: true, Descriptors: Config{PageSize: 2, Budget: budget}})
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	baseline := budget.Used()
	zero, e := a.Allocate(0)
	if e != nil || zero != (SmallBytesRef{}) || budget.Used() != baseline {
		t.Fatal("diagnostic zero allocation")
	}
	var refs [4]SmallBytesRef
	sizes := [4]uint32{1, 13, 247, 248}
	var requested uint64
	for i, n := range sizes {
		refs[i] = smallAlloc(t, a, n)
		requested += uint64(n)
		a.Write(refs[i], n-1, byte(i+31))
	}
	s := a.Stats()
	if s.SourceAllocationBytes != 0 || s.RequestedBytes != requested || s.DirectBytes != requested || s.DirectObjects != 4 || s.Chunks != 0 || s.FreeObjects != 0 || s.WastedBytes != 0 {
		t.Fatal("debug counters/pooling", s)
	}
	for i, r := range refs {
		if got, e := a.Read(r, sizes[i]-1); e != nil || got != byte(i+31) {
			t.Fatal("diagnostic payload content")
		}
	}
	before := budget.Used()
	smallFree(t, a, 13, refs[1])
	if before-budget.Used() != 13 {
		t.Fatal("debug small block not immediately released")
	}
	s = a.Stats()
	if s.SourceAllocationBytes != 0 || s.FreeObjects != 0 || s.WastedBytes != 0 || s.RequestedBytes != requested-13 {
		t.Fatal("debug free counter", s)
	}
	loc := smallLocation(t, a, refs[3])
	if e = a.Consolidate(); e != nil {
		t.Fatal(e)
	}
	if smallLocation(t, a, refs[3]) != loc {
		t.Fatal("debug consolidate affected direct block")
	}
	a.Reset()
	if a.IsLive(refs[3]) || a.Stats().SourceAllocationBytes != 0 || a.Stats().RequestedBytes != 0 {
		t.Fatal("debug reset cleanup")
	}
	next := smallAlloc(t, a, 1)
	if next == refs[0] {
		t.Fatal("debug stale generation resurrected")
	}
	a.Close()
	if budget.Used() != 0 {
		t.Fatal("debug close leaked")
	}
}
func TestSmallBytesReleaseSourceCounter(t *testing.T) {
	a := newSmallTest(t, 70, nil)
	small := smallAlloc(t, a, 13)
	large := smallAlloc(t, a, 248)
	if a.Stats().SourceAllocationBytes != 261 {
		t.Fatal("release source counter", a.Stats())
	}
	smallFree(t, a, 13, small)
	if a.Stats().SourceAllocationBytes != 248 {
		t.Fatal("release source free counter")
	}
	smallFree(t, a, 248, large)
	if a.Stats().SourceAllocationBytes != 0 {
		t.Fatal("release direct free counter")
	}
}
func TestSmallBytesDirectOnlyFailureAtomic(t *testing.T) {
	for failure := int64(0); failure < 8; failure++ {
		b := NewBudget(1 << 20)
		a, e := NewSmallBytes(SmallBytesConfig{DirectOnly: true, Descriptors: Config{PageSize: 2, Budget: b}})
		if e != nil {
			t.Fatal(e)
		}
		old := smallAlloc(t, a, 13)
		a.Write(old, 0, 117)
		b.FailAfter(failure)
		next, e := a.Allocate(1)
		if e != nil {
			if !errors.Is(e, ErrInjectedFailure) {
				t.Fatal(e)
			}
			if a.Stats().Live != 1 || a.Stats().RequestedBytes != 13 {
				t.Fatal("debug failure published")
			}
		} else {
			smallFree(t, a, 1, next)
		}
		if v, e := a.Read(old, 0); e != nil || v != 117 {
			t.Fatal("debug failure changed existing content")
		}
		if s := a.Stats(); s.SourceAllocationBytes != 0 || s.FreeObjects != 0 || s.WastedBytes != 0 {
			t.Fatal("debug failure source counters", s)
		}
		b.FailAfter(-1)
		a.Close()
		if b.Used() != 0 {
			t.Fatal("debug failure leaked")
		}
	}
}
