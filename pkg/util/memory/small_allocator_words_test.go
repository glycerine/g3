package memory

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// A native word payload belongs to the object generation it was attached
// for. An object that never had AttachWords, including one that reuses the
// location slot of a freed object, has no payload: Words must report
// ErrRuntimeUninitialized (reading a digit array that was never initialized),
// not hand out the previous occupant's words. Regression test for the
// upolynomial convert-bounds "actual read stops" failure after digits moved
// to native payloads.
func TestSmallAllocatorWordsRequireAttachedGeneration(t *testing.T) {
	r, e := NewRuntime(RuntimeConfig{Budget: arena.NewBudget(1 << 24), SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r})
	if e != nil {
		t.Fatal(e)
	}
	x, e := a.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := a.Words(x); !errors.Is(e, ErrRuntimeUninitialized) {
		t.Fatalf("payload before AttachWords: %v", e)
	}
	if e = a.AttachWords(x, 6); e != nil {
		t.Fatal(e)
	}
	w, e := a.Words(x)
	if e != nil || len(w) != 6 {
		t.Fatal("attached payload", len(w), e)
	}
	w[2] = 0xdead
	if e = a.Deallocate(32, x); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Words(x); e == nil {
		t.Fatal("freed object still has a payload")
	}
	// A plain object of the same size class takes the freed location.
	y, e := a.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	if y.slot != x.slot {
		t.Skipf("allocator did not reuse location %d (got %d)", x.slot, y.slot)
	}
	if y.generation == x.generation {
		t.Fatal("reused location kept its generation")
	}
	if _, e := a.Words(y); !errors.Is(e, ErrRuntimeUninitialized) {
		t.Fatalf("stale payload handed to the new occupant: %v", e)
	}
	// Attaching for the new occupant replaces the stale words.
	if e = a.AttachWords(y, 3); e != nil {
		t.Fatal(e)
	}
	w, e = a.Words(y)
	if e != nil || len(w) != 3 || w[2] != 0 {
		t.Fatal("reattached payload not reset", w, e)
	}
}
