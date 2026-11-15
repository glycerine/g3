package memory

// Tests-first concrete shared memory owner contract; no duplicate implementation.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func memoryAPIMust(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

func TestMemorySharedRuntimeAllocatorIdentity(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	memoryAPIMust(t, e)
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	c, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	if a.RuntimeOwner() != r || c.RuntimeOwner() != r || r.OwnerID() == 0 {
		t.Fatal("owners do not share the actual runtime")
	}
	if r.Stats().LiveBlocks != 0 || r.State().GlobalCount != 0 {
		t.Fatal("cold owners invented source allocations", r.Stats(), r.State())
	}
	x, e := a.Allocate(13)
	memoryAPIMust(t, e)
	y, e := c.Allocate(13)
	memoryAPIMust(t, e)
	hx, ox, e := a.RawLocation(x)
	memoryAPIMust(t, e)
	hy, oy, e := c.RawLocation(y)
	memoryAPIMust(t, e)
	if hx.OwnerID() != r.OwnerID() || hy.OwnerID() != r.OwnerID() || hx == hy || ox != 16 || oy != 16 {
		t.Fatal("scalar locations are not genuine distinct runtime chunks", hx, hy, ox, oy)
	}
	if r.State().GlobalCount != 2 || r.State().GlobalSize != 16400 || r.Stats().PhysicalBytes != 16400 || r.Stats().LiveBlocks != 2 {
		t.Fatal("actual shared physical events", r.State(), r.Stats())
	}
	memoryAPIMust(t, a.Write(x, 12, 7))
	v, e := r.Read(hx, ox+12)
	memoryAPIMust(t, e)
	if v != 7 {
		t.Fatal("allocator does not use the runtime backing", v)
	}
	memoryAPIMust(t, a.Deallocate(13, x))
	memoryAPIMust(t, c.Deallocate(13, y))
	n, e := a.AllocationSize()
	memoryAPIMust(t, e)
	if n != 0 || r.State().GlobalCount != 2 || r.Stats().PhysicalBytes != 16400 {
		t.Fatal("recycle fabricated raw frees", n, r.State(), r.Stats())
	}
	memoryAPIMust(t, a.Close())
	if r.Stats().LiveBlocks != 1 {
		t.Fatal("owner freed another allocator's chunk", r.Stats())
	}
	memoryAPIMust(t, c.Close())
	memoryAPIMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("physical owner graph retained", b.Used())
	}
}

func TestMemorySharedSourceProfiles(t *testing.T) {
	for _, bits := range []uint8{32, 64} {
		for _, mode := range [][2]bool{{false, false}, {true, false}, {true, true}} {
			b := arena.NewBudget(1 << 20)
			r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: bits})
			memoryAPIMust(t, e)
			p := SmallAllocatorProfile{PointerBits: bits, Debug: mode[0], Windows: mode[1]}
			a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: p})
			memoryAPIMust(t, e)
			width := uint64(bits / 8)
			threshold := 256 - width
			sizes := [3]uint64{1, threshold - 1, threshold}
			var refs [3]ObjectRef
			for i, n := range sizes {
				refs[i], e = a.Allocate(n)
				memoryAPIMust(t, e)
			}
			requested, e := a.AllocationSize()
			memoryAPIMust(t, e)
			physical := 2*(8192+width) + threshold + width
			chunks, direct := uint32(2), uint32(1)
			if p.Debug && !p.Windows {
				requestedWant := uint64(0)
				if requested != requestedWant {
					t.Fatal("debug bypass requested counter", bits, requested)
				}
				physical = 2*threshold + 3*width
				chunks, direct = 0, 3
			} else if requested != 2*threshold {
				t.Fatal("source requested count", bits, p, requested)
			}
			if r.Stats().PhysicalBytes != physical || r.State().GlobalCount != 3 || a.Stats().Chunks != chunks || a.Stats().DirectObjects != direct {
				t.Fatal("profile physical topology", bits, p, r.State(), r.Stats(), a.Stats(), physical)
			}
			for i, n := range sizes {
				memoryAPIMust(t, a.Deallocate(n, refs[i]))
			}
			remaining := uint64(chunks) * (8192 + width)
			if r.Stats().PhysicalBytes != remaining || r.State().GlobalSize != int64(remaining) {
				t.Fatal("source direct versus pooled release", bits, p, r.Stats(), r.State())
			}
			memoryAPIMust(t, a.Close())
			memoryAPIMust(t, r.Close())
			if b.Used() != 0 {
				t.Fatal("profile ownership teardown", b.Used())
			}
		}
	}
}

func TestMemorySharedTLSLaneGetterAndCumulativeCount(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true})
	memoryAPIMust(t, e)
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	c, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	x, e := a.Allocate(32)
	memoryAPIMust(t, e)
	y, e := c.Allocate(32)
	memoryAPIMust(t, e)
	n, e := r.AllocationSize()
	memoryAPIMust(t, e)
	if n != 0 || r.State().LocalSize != 16400 || r.State().LocalCount != 2 || r.State().GlobalCount != 0 {
		t.Fatal("getter flushed shared TLS lane", n, r.State())
	}
	large, e := a.Allocate(100000)
	memoryAPIMust(t, e)
	if r.State().GlobalSize != 116408 || r.State().GlobalCount != 3 || r.State().LocalSize != 0 || r.State().LocalCount != 3 {
		t.Fatal("actual threshold synchronization", r.State())
	}
	memoryAPIMust(t, a.Deallocate(100000, large))
	if r.State().GlobalSize != 16400 || r.State().GlobalCount != 6 || r.State().LocalSize != 0 || r.State().LocalCount != 3 {
		t.Fatal("cumulative count was reset or not contributed", r.State())
	}
	memoryAPIMust(t, a.Deallocate(32, x))
	memoryAPIMust(t, c.Deallocate(32, y))
	memoryAPIMust(t, a.Close())
	memoryAPIMust(t, c.Close())
	n, e = r.AllocationSize()
	memoryAPIMust(t, e)
	if n != 16400 || r.State().LocalSize != -16400 || r.Stats().LiveBlocks != 0 {
		t.Fatal("teardown invented a getter flush", n, r.State(), r.Stats())
	}
	memoryAPIMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("TLS physical metadata teardown", b.Used())
	}
}

func TestMemorySharedRawRefusalRetainsSourcePrefix(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	memoryAPIMust(t, e)
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	memoryAPIMust(t, r.SetMaxSize(1))
	ref, e := a.Allocate(7)
	var f Failure
	if !errors.As(e, &f) || f.ErrorCode() != 101 || ref != (ObjectRef{}) {
		t.Fatal("actual source memout classification", ref, e)
	}
	n, e := a.AllocationSize()
	memoryAPIMust(t, e)
	if n != 7 || r.State().GlobalSize != 8200 || r.State().GlobalCount != 1 || r.Stats().LiveBlocks != 0 {
		t.Fatal("requested/raw counter publication prefix", n, r.State(), r.Stats())
	}
	memoryAPIMust(t, a.Close())
	memoryAPIMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("refusal metadata cleanup", b.Used())
	}
}

// One borrowed cold receiver, not a closure or per-record box. This instrument
// observes dispatch/root timing only; it is not an ostream implementation.
type memoryAPIIO struct {
	runtime *Runtime
	calls   int
	event   AllocatorIOEvent
	sawFree bool
}

func (c *memoryAPIIO) Emit(a *SmallAllocator, event AllocatorIOEvent) error {
	c.calls++
	c.event = event
	c.sawFree = c.runtime.Stats().LiveBlocks == 0
	if a.RuntimeOwner() != c.runtime {
		return errors.New("different runtime passed to cold dispatcher")
	}
	return nil
}

type memoryAPIBridgedFailure struct{ failure Failure }

func (e memoryAPIBridgedFailure) Error() string { return e.failure.Error() }
func memoryAPITransport(f Failure) error        { return memoryAPIBridgedFailure{f} }

func TestMemoryBorrowedStaticErrorAndAllocatorIO(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64, Transport: memoryAPITransport})
	memoryAPIMust(t, e)
	io := memoryAPIIO{runtime: r}
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64, Debug: true, Windows: true}, IO: &io})
	memoryAPIMust(t, e)
	if io.calls != 0 {
		t.Fatal("constructor emitted diagnostic")
	}
	_, e = a.Allocate(13)
	memoryAPIMust(t, e)
	memoryAPIMust(t, a.Close())
	if io.calls != 1 || io.event != AllocatorIOLeak || !io.sawFree {
		t.Fatal("diagnostic dispatch before actual chunk frees", io)
	}
	memoryAPIMust(t, a.Close())
	if io.calls != 1 {
		t.Fatal("diagnostic replayed", io.calls)
	}
	c, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	memoryAPIMust(t, r.SetMaxSize(1))
	_, e = c.Allocate(7)
	var f memoryAPIBridgedFailure
	if !errors.As(e, &f) || f.failure.ErrorCode() != 101 {
		t.Fatal("static source transport not invoked", e)
	}
	memoryAPIMust(t, c.Close())
	memoryAPIMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("borrowed transport ownership charge", b.Used())
	}
}

func TestMemoryScalarLocationRecords(t *testing.T) {
	memoryAPIMust(t, arena.ValidateRecordType(reflect.TypeOf(Block{})))
	memoryAPIMust(t, arena.ValidateRecordType(reflect.TypeOf(ObjectRef{})))
	var b Block
	var o ObjectRef
	if b.OwnerID() != 0 || b.Slot() != 0 || b.Generation() != 0 || o.OwnerID() != 0 || o.Slot() != 0 || o.Generation() != 0 || o.LogicalAddress() != 0 {
		t.Fatal("zero scalar token")
	}
}

func memoryAPINilTransport(Failure) error { return nil }
func TestMemoryNilTransportCannotSuppressFailure(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64, Transport: memoryAPINilTransport})
	memoryAPIMust(t, e)
	var h Block
	t.Cleanup(func() {
		if h != (Block{}) {
			_ = r.Deallocate(&h)
		}
		_ = r.Close()
	})
	memoryAPIMust(t, r.SetMaxSize(1))
	h, e = r.Allocate(1)
	if e != ErrFailureTransport || h != (Block{}) || r.Stats().LiveBlocks != 0 || r.State().GlobalSize != 9 || r.State().GlobalCount != 1 {
		t.Fatal("nil static transport suppressed real failure prefix", h, e, r.Stats(), r.State())
	}
	memoryAPIMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("transport refusal metadata cleanup", b.Used())
	}
}

// TestMemoryAnyFailureTransportActiveCoversEveryRuntime checks the Go-only
// process-wide shortcut: it is set exactly while some Runtime's transport runs.
func TestMemoryAnyFailureTransportActiveCoversEveryRuntime(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	var r *Runtime
	seenOwn, seenAny := false, false
	transport := func(f Failure) error {
		seenOwn, seenAny = r.FailureTransportActive(), AnyFailureTransportActive()
		return memoryAPIBridgedFailure{f}
	}
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64, Transport: transport})
	memoryAPIMust(t, e)
	if AnyFailureTransportActive() {
		t.Fatal("transport reported outside any transport")
	}
	c, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: 64}})
	memoryAPIMust(t, e)
	memoryAPIMust(t, r.SetMaxSize(1))
	_, e = c.Allocate(7)
	var f memoryAPIBridgedFailure
	if !errors.As(e, &f) || !seenOwn || !seenAny {
		t.Fatal("transport not observed", e, seenOwn, seenAny)
	}
	if AnyFailureTransportActive() || r.FailureTransportActive() {
		t.Fatal("transport still reported after it returned")
	}
	memoryAPIMust(t, c.Close())
	memoryAPIMust(t, r.Close())
}
