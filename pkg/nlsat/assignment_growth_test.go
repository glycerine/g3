package nlsat

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func TestAssignmentGrowthUsesAvailableBudget(t *testing.T) {
	m, budget := assignmentFixture(t)
	a := assignmentNew(t, m)
	initial := budget.Used()
	var value polynomial.Algebraic
	// A sparse first assignment can allocate its cells and then fail on flags.
	held := budget.Limit() - budget.Used() - 4*assignmentValueBytes
	assignmentMust(t, budget.Reserve(held))
	err := a.SetCore(3, &value)
	budget.Release(held)
	if !errors.Is(err, arena.ErrBudget) || len(a.values) != 4 || len(a.assigned) != 0 {
		t.Fatalf("partial growth left %d cells and %d flags, error %v", len(a.values), len(a.assigned), err)
	}
	assignmentMust(t, a.SetCore(3, &value))

	// Each replacement has room for the exact five entries. Both old
	// arrays remain charged until their respective replacements exist.
	held = budget.Limit() - budget.Used() - 5*assignmentValueBytes
	assignmentMust(t, budget.Reserve(held))
	err = a.growValues(5)
	budget.Release(held)
	assignmentMust(t, err)
	held = budget.Limit() - budget.Used() - 5
	assignmentMust(t, budget.Reserve(held))
	err = a.growFlags(5)
	budget.Release(held)
	assignmentMust(t, err)
	if cap(a.values) != 5 || cap(a.assigned) != 5 || !a.assigned[3] || a.assigned[4] {
		t.Fatalf("exact growth left capacities %d/%d and flags %v", cap(a.values), cap(a.assigned), a.assigned)
	}

	assignmentMust(t, a.SetCore(5, &value))
	if cap(a.values) <= len(a.values) || cap(a.assigned) <= len(a.assigned) {
		t.Fatal("sequential assignment lost spare capacity")
	}
	want := initial + uint64(cap(a.values))*assignmentValueBytes + uint64(cap(a.assigned))
	if got := budget.Used(); got != want {
		t.Fatalf("used %d bytes, want %d including spare capacity", got, want)
	}
	assignmentMust(t, a.Reset())
	assignmentMust(t, a.SetCore(6, &value))
	if a.assigned[3] || a.assigned[5] || !a.assigned[6] {
		t.Fatalf("reused flags after reset %v", a.assigned)
	}
}

func BenchmarkNLSATAssignmentGrowth(b *testing.B) {
	m, _ := assignmentFixture(b)
	for _, count := range []int{128, 1024, 8192} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			var value polynomial.Algebraic
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				a, err := NewAssignment(m)
				if err != nil {
					b.Fatal(err)
				}
				for id := 0; id < count; id++ {
					if err := a.SetCore(uint32(id), &value); err != nil {
						b.Fatal(err)
					}
				}
				if err := a.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
