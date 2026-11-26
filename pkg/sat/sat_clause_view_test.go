package sat

import (
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func satViewAllocator(t *testing.T) *ClauseAllocator {
	t.Helper()
	a, err := NewSATClauseAllocator(satClauseAllocatorConfig{Storage: arena.SmallBytesConfig{Descriptors: arena.Config{Budget: arena.NewBudget(1 << 24)}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// The propagation view must read and write exactly what the checked accessors do.
func TestClauseViewMatchesCheckedAccessors(t *testing.T) {
	a := satViewAllocator(t)
	r := rand.New(rand.NewSource(1))
	for round := 0; round < 200; round++ {
		n := 2 + r.Intn(30)
		literals := make([]Literal, n)
		for i := range literals {
			literals[i] = Literal(uint32(r.Intn(1000)))
		}
		learned := r.Intn(2) == 0
		c, err := a.MkClause(literals, learned)
		if err != nil {
			t.Fatal(err)
		}
		view, err := a.view(c)
		if err != nil {
			t.Fatal(err)
		}
		size, err := a.Size(c)
		if err != nil || view.size() != size || size != uint32(n) {
			t.Fatal("size", view.size(), size, n, err)
		}
		for i := uint32(0); i < size; i++ {
			got, err := a.Literal(c, i)
			if err != nil || view.literal(i) != got {
				t.Fatal("literal", i, view.literal(i), got, err)
			}
		}
		// Writes through the view are visible to the accessors, and back.
		view.setLiteral(1, Literal(777))
		if got, _ := a.Literal(c, 1); got != Literal(777) {
			t.Fatal("view write not visible", got)
		}
		if err = a.SetLiteral(c, 0, Literal(555)); err != nil || view.literal(0) != Literal(555) {
			t.Fatal("accessor write not visible", view.literal(0), err)
		}
		// Flags must not disturb one another.
		removed, _ := a.Removed(c)
		if view.flag(satClauseFlagRemoved) != removed {
			t.Fatal("removed flag")
		}
		wasLearned, _ := a.Learned(c)
		view.setFlag(satClauseFlagUsed)
		used, _ := a.Used(c)
		nowLearned, _ := a.Learned(c)
		if !used || nowLearned != wasLearned || nowLearned != learned {
			t.Fatal("used flag", used, nowLearned, wasLearned)
		}
		if err = a.SetRemoved(c, true); err != nil || !view.flag(satClauseFlagRemoved) {
			t.Fatal("removed flag through accessor", err)
		}
	}
}

// Large clauses live in a separate allocation, reached by the same view.
func TestClauseViewReadsLargeStorage(t *testing.T) {
	a := satViewAllocator(t)
	r, err := a.allocateLarge(9 * 4)
	if err != nil {
		t.Fatal(err)
	}
	view, err := a.view(r)
	if err != nil {
		t.Fatal(err)
	}
	view.setWord(1, 4)
	view.setLiteral(2, Literal(99))
	view.setFlag(satClauseFlagRemoved)
	if view.size() != 4 || view.literal(2) != Literal(99) || !view.flag(satClauseFlagRemoved) || view.flag(satClauseFlagUsed) {
		t.Fatal("large view", view)
	}
	if got, err := a.readWord(r, 7); err != nil || got != 99 {
		t.Fatal("large word through checked path", got, err)
	}
	if err := a.writeWord(r, 7, 101); err != nil || view.literal(2) != 101 {
		t.Fatal("checked wide write", err, view.literal(2))
	}
	if err := a.releaseLarge(r); err != nil {
		t.Fatal(err)
	}
	if _, err := a.view(r); err == nil {
		t.Fatal("released wide clause accepted")
	}
}

func TestClauseViewRefusesInvalidReferences(t *testing.T) {
	a, other := satViewAllocator(t), satViewAllocator(t)
	c, err := a.MkClause([]Literal{Literal(2), Literal(4), Literal(6)}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.view(c); err == nil {
		t.Fatal("foreign clause accepted")
	}
	if _, err = a.view(StoredClauseRef{owner: a.origin, storageKind: 9, slot: 1, generation: 1}); err == nil {
		t.Fatal("unknown storage kind accepted")
	}
	if _, err = a.view(StoredClauseRef{owner: a.origin}); err == nil {
		t.Fatal("null clause accepted")
	}
	stale := c
	if err = a.Delete(stale); err != nil {
		t.Fatal(err)
	}
	if _, err = a.view(stale); err == nil {
		t.Fatal("deallocated clause accepted")
	}
}

// A clause visit must not allocate.
func TestClauseViewDoesNotAllocate(t *testing.T) {
	a := satViewAllocator(t)
	c, err := a.MkClause([]Literal{Literal(2), Literal(4), Literal(6)}, false)
	if err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(200, func() {
		view, _ := a.view(c)
		view.setLiteral(0, view.literal(1))
		view.setFlag(satClauseFlagUsed)
	}); allocs != 0 {
		t.Fatalf("%v allocations", allocs)
	}
}

func TestClauseViewMoveWatchPreservesLiteralOrder(t *testing.T) {
	for _, width := range []int{2, 5, 6, 7, 10, 17, 18, 19, 64} {
		view := satClauseView(make([]byte, satClauseLiteralsOffset+4*width))
		view.setWord(1, uint32(width))
		assignment := make([]lbool.LBool, 2*width)
		// Exercise every return position, including a false tail and the
		// transition from a complete group to the remaining literals.
		for first := 2; first <= width; first++ {
			for index := range assignment {
				assignment[index] = lbool.False
			}
			for index := range width {
				view.setLiteral(uint32(index), Literal(2*index))
			}
			if first < width {
				assignment[2*(width-1)] = lbool.True
				assignment[2*first] = lbool.Undef
			}
			const falsified = Literal(2)
			replacement, moved := view.moveWatch(falsified, assignment)
			if moved != (first < width) || moved && replacement != Literal(2*first) {
				t.Fatalf("width %d, first candidate at %d: got (%d, %t)", width, first, replacement, moved)
			}
			for index := range width {
				want := Literal(2 * index)
				switch {
				case moved && index == 1:
					want = replacement
				case moved && index == first:
					want = falsified
				}
				if got := view.literal(uint32(index)); got != want {
					t.Fatalf("width %d, first candidate at %d, slot %d: want %d, got %d", width, first, index, want, got)
				}
			}
		}
	}
}
