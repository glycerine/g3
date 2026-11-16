package numeral

// Source: src/util/mpf.h:241-290; src/util/mpf.cpp:1821-1847.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatPowersRuntimeWrapperCacheAndSourceDestruction(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	for _, n := range []uint32{0, 30, 31, 100} {
		w, err := m.powerTwoWrapper(n, false)
		serviceMust(t, err)
		a, err := w.Capture()
		serviceMust(t, err)
		if n < 31 && a != NewInt(int32(uint32(1)<<n)) {
			t.Fatal("literal power value", n, a)
		}
		extent, err := r.Size(w.Block)
		serviceMust(t, err)
		if extent != 16 {
			t.Fatal("sole source wrapper extent", extent)
		}
		before := b.Used()
		b.FailAfter(0)
		again, err := m.powerTwoWrapper(n, false)
		serviceMust(t, err)
		borrowed, err := m.BorrowPowerOfTwo(n)
		serviceMust(t, err)
		if borrowed != a {
			t.Fatal("borrow must expose the cached NUM, not an owned copy", n)
		}
		if again != w || b.Used() != before {
			t.Fatal("source cache hit allocated or replaced wrapper")
		}
		b.FailAfter(-1)
	}
	serviceMust(t, m.SourceFinalize())
	if _, err := m.BorrowPowerOfTwo(0); err == nil {
		t.Fatal("ended owner lent a cached power")
	}
	if r.Stats().LiveBlocks != 0 {
		t.Fatal("source power wrapper/NUM destruction", r.Stats())
	}
	serviceMust(t, m.Close())
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("exact manager context end", b.Used())
	}
}

func TestArbitraryFloatPowerWrapperEveryActualReservationPrefix(t *testing.T) {
	failed, completed := false, false
	for ordinal := int64(0); ordinal < 64; ordinal++ {
		b := arena.NewBudget(1 << 28)
		r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
		serviceMust(t, e)
		m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
		serviceMust(t, e)
		foreign, e := r.Allocate(23)
		serviceMust(t, e)
		b.FailAfter(ordinal)
		_, call := m.powerTwoWrapper(100, false)
		var raw []umemory.Block
		var nums []*Int
		values := make([]Int, 0, 8)
		raw = append(raw, foreign)
		for w := m.PowerWrappers; w != nil; w = w.Next {
			if w.Initialized {
				a, read := w.Capture()
				serviceMust(t, read)
				values = append(values, a)
			}
		}
		for i := range values {
			nums = append(nums, &values[i])
		}
		integerPhysicalAuditWithRaw(t, m.integer, raw, nums...)
		if b.Used() != r.Stats().TotalBytes+m.OwnedStorageBytes() {
			t.Fatal("before-lift named cold graph", ordinal, call, b.Used(), r.Stats().TotalBytes, m.OwnedStorageBytes())
		}
		b.FailAfter(-1)
		if call != nil {
			failed = true
		} else {
			completed = true
		}
		if e = m.Close(); e != nil {
			serviceMust(t, m.Close())
		}
		if _, e = r.Size(foreign); e != nil {
			t.Fatal("borrowed foreign root released", ordinal, e)
		}
		serviceMust(t, r.Deallocate(&foreign))
		serviceMust(t, r.Close())
		if b.Used() != 0 {
			t.Fatal("owning end", ordinal, b.Used())
		}
		if call == nil {
			break
		}
	}
	if !failed || !completed {
		t.Fatal("actual allocation horizon not completed", failed, completed)
	}
}

func TestArbitraryFloatPowerMinusOneAndNegatedCaches(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	for _, n := range []uint32{0, 1, 30, 31, 32, 62} {
		for _, neg := range []bool{false, true} {
			w, err := m.powerTwoMinusOneWrapper(n, neg)
			serviceMust(t, err)
			a, err := w.Capture()
			serviceMust(t, err)
			value, err := m.integer.Int64(&a)
			serviceMust(t, err)
			expected := int64((uint64(1) << n) - 1)
			if neg {
				expected = -expected
			}
			if value != expected {
				t.Fatal("independent bounded m1 oracle", n, neg, value, expected)
			}
		}
	}
	serviceMust(t, m.SourceFinalize())
	serviceMust(t, m.Close())
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("actual four-cache context end", b.Used())
	}
}

func TestArbitraryFloatPowerActualAllocationCallbackPrefixAndOwnerExclusion(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var r *umemory.Runtime
	var m *ArbitraryFloatManager
	active, calls := false, 0
	var seen uint32
	var lifecycle []error
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true, Transport: func(f umemory.Failure) error {
		if active {
			calls++
			w := m.PowerWrappers
			var read error
			seen, read = r.ReadUint32(w.Block, 0)
			serviceMust(t, read)
			lifecycle = append(lifecycle, m.Close(), m.SourceFinalize(), m.integer.CloseChecked(), m.integer.SourceFinalize(), m.rational.CloseChecked(), m.rational.SourceFinalize())
		}
		return f
	}})
	serviceMust(t, e)
	m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	if r.State().LocalSize >= 99900 {
		t.Fatal("fixture TLS constructor extent", r.State())
	}
	padding, e := r.Allocate(uint64(99900 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	active = true
	_, call := m.powerTwoWrapper(1000000, false)
	active = false
	if call == nil || calls == 0 || seen != 1 {
		t.Fatal("actual allocation sign prefix", call, calls, seen)
	}
	for _, err := range lifecycle {
		if err != ErrNumericPending {
			t.Fatal("executing actual owner lifecycle", err)
		}
	}
	w := m.PowerWrappers
	if w == nil || !w.published || w.bodyCause == nil || w.Roots == (NumericAllocationRoots{}) {
		t.Fatal("actual returned ownership prefix", w)
	}
	a, e := w.Capture()
	serviceMust(t, e)
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{padding}, &a)
	if b.Used() != r.Stats().TotalBytes+m.OwnedStorageBytes() {
		t.Fatal("before-lift exact physical/cold graph", b.Used())
	}
	serviceMust(t, r.SetMaxSize(0))
	if e = m.Close(); e != nil {
		serviceMust(t, m.Close())
	}
	serviceMust(t, r.Deallocate(&padding))
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("exact own context end", b.Used())
	}
}

func TestArbitraryFloatPowerContextRejectsIdleCopiedAndStaleReceipts(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	w, e := m.powerTwoWrapper(8, false)
	serviceMust(t, e)
	before, state := b.Used(), r.State()
	copied := *w
	two := NewInt(2)
	for _, c := range []integerAllocationInvocationContext{
		{mpfPower: w, mpfPowerInvocation: w.invocation},
		{mpfPower: w, mpfPowerInvocation: w.invocation + 1},
		{mpfPower: &copied, mpfPowerInvocation: w.invocation},
	} {
		out, e := w.Capture()
		serviceMust(t, e)
		call := m.integer.powerWithPhysicalAllocationContext(&two, 100, &out, nil, &c, w.receiver())
		if call != ErrNumericCallerOwner {
			t.Fatal("invalid source receipt accepted", call)
		}
		after, e := w.Capture()
		serviceMust(t, e)
		if after != NewInt(256) || b.Used() != before || r.State() != state {
			t.Fatal("invalid receipt mutated actual caller")
		}
	}
	serviceMust(t, m.Close())
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("owner end", b.Used())
	}
}

func TestArbitraryFloatMinusOneScratchFailureReadsPublishedPowerPrefix(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var r *umemory.Runtime
	var m *ArbitraryFloatManager
	active, calls := false, 0
	var oldPower Int
	var sawScratchHole bool
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true, Transport: func(f umemory.Failure) error {
		if active {
			calls++
			var read error
			oldPower, read = m.PowerWrappers.Capture()
			serviceMust(t, read)
			sawScratchHole = m.integer.Tmp[0] == 0
		}
		return f
	}})
	serviceMust(t, e)
	m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	// Power's 62520-byte source cell fits below the synchronization threshold;
	// the following literal subtraction scratch growth crosses it.
	target := int64(90000 - 62528)
	if r.State().LocalSize >= target {
		t.Fatal("actual TLS fixture extent", r.State())
	}
	padding, e := r.Allocate(uint64(target - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	active = true
	_, call := m.powerTwoMinusOneWrapper(500000, false)
	active = false
	if call == nil || calls != 1 || !sawScratchHole || oldPower.IsSmall() || oldPower.Val != 1 {
		t.Fatal("reached subtraction scratch prefix", call, calls, sawScratchHole, oldPower)
	}
	if m.integer.Cell(oldPower.Cell).Size != 15626 || m.integer.Digit(oldPower.Cell, 15625) != 1 {
		t.Fatal("literal Power completed before decrement scratch failure")
	}
	w := m.PowerWrappers
	after, e := w.Capture()
	serviceMust(t, e)
	if after != oldPower || w.bodyCause == nil || w.Roots == (NumericAllocationRoots{}) {
		t.Fatal("failed decrement changed sole wrapper or lost actual outcome", after, oldPower, w)
	}
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{padding}, &after)
	if b.Used() != r.Stats().TotalBytes+m.OwnedStorageBytes() {
		t.Fatal("before-lift exact scratch acquisition graph", b.Used())
	}
	serviceMust(t, r.SetMaxSize(0))
	if e = m.Close(); e != nil {
		serviceMust(t, m.Close())
	}
	serviceMust(t, r.Deallocate(&padding))
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("owning actual scratch and wrapper end", b.Used())
	}
}

func TestArbitraryFloatThreeFailedCacheMissesThenSuccessfulMissAndWarmHit(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	foreign, e := r.Allocate(23)
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	var stopped []*arbitraryFloatPowerWrapper
	for _, n := range []uint32{1000000, 1000010, 1000020} {
		_, call := m.powerTwoWrapper(n, false)
		w := m.PowerWrappers
		if call == nil || w == nil || !w.published || w.Transferred.Block() == (umemory.Block{}) {
			t.Fatal("actual failed cache acquisition", n, call, w)
		}
		stopped = append(stopped, w)
		integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{foreign})
		if b.Used() != r.Stats().TotalBytes+m.OwnedStorageBytes() {
			t.Fatal("before-lift independent failed cache graph", n, b.Used())
		}
	}
	original := []umemory.Block{stopped[0].Transferred.Block(), stopped[1].Transferred.Block(), stopped[2].Transferred.Block()}
	serviceMust(t, r.SetMaxSize(0))
	current, e := m.powerTwoWrapper(100, false)
	serviceMust(t, e)
	before := b.Used()
	b.FailAfter(0)
	again, e := m.powerTwoWrapper(100, false)
	serviceMust(t, e)
	if current != again || b.Used() != before {
		t.Fatal("warm cache overwrote or allocated")
	}
	for i, w := range stopped {
		if w.Transferred.Block() != original[i] || w.bodyCause == nil {
			t.Fatal("later source call overwrote exact older acquisition", i, w)
		}
	}
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{foreign})
	if b.Used() != r.Stats().TotalBytes+m.OwnedStorageBytes() {
		t.Fatal("warm before-lift graph", b.Used())
	}
	b.FailAfter(-1)
	serviceMust(t, m.SourceFinalize())
	for i, w := range stopped {
		if w.Transferred.Block() != original[i] {
			t.Fatal("source DT aborted unjoined acquisition", i, w)
		}
	}
	serviceMust(t, m.Close())
	for i, w := range stopped {
		if w.Transferred.Block() != (umemory.Block{}) || w.bodyCause == nil || w.original == (umemory.Block{}) {
			t.Fatal("own ACK lost original source history", i, w)
		}
	}
	if _, e = r.Size(foreign); e != nil {
		t.Fatal("foreign root lost", e)
	}
	serviceMust(t, r.Deallocate(&foreign))
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("true enclosing context end", b.Used())
	}
}

func TestArbitraryFloatPooledDescriptorFailureSourceEndThenOwnHistory(t *testing.T) {
	testArbitraryFloatDescriptorFailureSourceEndThenOwnHistory(t, true)
}
func TestArbitraryFloatDirectDescriptorFailureSourceEndThenOwnHistory(t *testing.T) {
	testArbitraryFloatDescriptorFailureSourceEndThenOwnHistory(t, false)
}
func testArbitraryFloatDescriptorFailureSourceEndThenOwnHistory(t *testing.T, pooled bool) {
	found := false
	for ordinal := int64(0); ordinal < 64; ordinal++ {
		b := arena.NewBudget(1 << 28)
		r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
		serviceMust(t, e)
		m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: !pooled}, Cells: arena.Config{Budget: b, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
		serviceMust(t, e)
		foreign, e := r.Allocate(23)
		serviceMust(t, e)
		b.FailAfter(ordinal)
		_, call := m.powerTwoWrapper(100, false)
		w := m.PowerWrappers
		selected := call != nil && w != nil && w.Roots.Cell.Object != (umemory.ObjectRef{}) && (w.Roots.Cell.Offset != 0) == pooled
		integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{foreign})
		if b.Used() != r.Stats().TotalBytes+m.OwnedStorageBytes() {
			t.Fatal("before-lift exact descriptor-prefix graph", ordinal, b.Used())
		}
		b.FailAfter(-1)
		if selected {
			found = true
			original := w.originalCell
			body := w.bodyCause
			serviceMust(t, m.SourceFinalize())
			if !m.sourceEnded || !w.SourceDelDone || !w.sourceFreeDone || body == nil || w.originalCell != original {
				t.Fatal("actual completed source suffix/history", ordinal, w)
			}
			if pooled {
				if _, read := r.ReadUint32(original.Raw, original.Offset+4); read != umemory.ErrRuntimeOwner {
					t.Fatal("actual pooled source release", ordinal, read)
				}
				if !w.cellRetired || w.Roots.Cell.Object != (umemory.ObjectRef{}) {
					t.Fatal("actual physical retirement stamp", ordinal, w)
				}
			} else {
				if _, read := r.Size(original.Raw); read != nil {
					t.Fatal("actual detached direct allocation source survival", ordinal, read)
				}
				if w.cellRetired || w.Roots.Cell != original {
					t.Fatal("source invented direct cleanup", ordinal, w)
				}
			}
			if own := m.Close(); own != nil {
				t.Fatal("own replayed actual source-ended pooled token", ordinal, own)
			}
			if w.originalCell != original || w.bodyCause != body {
				t.Fatal("ACK erased immutable constructor tuple", ordinal, w)
			}
		} else {
			if e = m.Close(); e != nil {
				if second := m.Close(); second != nil {
					t.Fatal("earlier nonselected owner end", ordinal, e, second)
				}
			}
		}
		if _, e = r.Size(foreign); e != nil {
			t.Fatal("borrowed foreign root ended", ordinal, e)
		}
		serviceMust(t, r.Deallocate(&foreign))
		serviceMust(t, r.Close())
		if b.Used() != 0 {
			t.Fatal("context end", ordinal, b.Used())
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("genuine pooled descriptor-preparation failure not reached")
	}
}
