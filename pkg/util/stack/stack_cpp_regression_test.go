package stack

// Source: src/test/stack.cpp tst1 and tst2.
import (
	"fmt"
	"testing"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestStackCPPRegression(t *testing.T) {
	t.Run("placement", func(t *testing.T) {
		s, err := NewStack()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		}()
		first, err := s.Allocate(8)
		if err != nil {
			t.Fatal(err)
		}
		second, err := s.Allocate(8)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Write(first, 0, 10); err != nil {
			t.Fatal(err)
		}
		if err = s.Write(first, 4, 20); err != nil {
			t.Fatal(err)
		}
		if err = s.Write(second, 0, 30); err != nil {
			t.Fatal(err)
		}
		if err = s.Write(second, 4, 40); err != nil {
			t.Fatal(err)
		}
		large, err := s.Allocate(16000)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			ref    StackRef
			offset uint32
			want   byte
		}{{first, 0, 10}, {first, 4, 20}, {second, 0, 30}, {second, 4, 40}} {
			got, err := s.Read(item.ref, item.offset)
			if err != nil || got != item.want {
				t.Fatalf("allocation = %d, %v; want %d", got, err, item.want)
			}
		}
		for _, ref := range []StackRef{large, second, first} {
			if err = s.DeallocateRef(ref); err != nil {
				t.Fatal(err)
			}
		}
	})
	for _, row := range []struct{ count, rate uint32 }{{1000, 10}, {2000, 2}, {100000, 10}, {300000, 5}, {300000, 2}, {300000, 7}} {
		t.Run(fmt.Sprintf("%d/%d", row.count, row.rate), func(t *testing.T) {
			s, err := NewStack()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := s.Close(); err != nil {
					t.Error(err)
				}
			}()
			rng := z3rand.New(0)
			var refs []StackRef
			for i := uint32(0); i < row.count; i++ {
				if s.Empty() != (len(refs) == 0) {
					t.Fatalf("empty mismatch at %d", i)
				}
				if len(refs) != 0 {
					top, err := s.Top()
					if err != nil || top != refs[len(refs)-1] {
						t.Fatalf("top mismatch at %d: %v", i, err)
					}
				}
				if len(refs) != 0 && rng.Bounded(row.rate) == 0 {
					if err := s.Deallocate(); err != nil {
						t.Fatal(err)
					}
					refs = refs[:len(refs)-1]
				} else {
					size := rng.Bounded(100)
					if rng.Bounded(10) == 0 {
						size = 8192 + rng.Bounded(800)
					}
					ref, err := s.Allocate(size)
					if err != nil {
						t.Fatal(err)
					}
					refs = append(refs, ref)
				}
			}
			// The C++ loop uses while (s.empty()); release live allocations instead.
			for len(refs) != 0 {
				if err := s.DeallocateRef(refs[len(refs)-1]); err != nil {
					t.Fatal(err)
				}
				refs = refs[:len(refs)-1]
			}
			if !s.Empty() {
				t.Fatal("stack did not drain")
			}
		})
	}
}
