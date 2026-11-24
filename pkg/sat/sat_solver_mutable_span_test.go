package sat

// Source: src/sat/sat_solver.cpp:367–465,161–176; src/util/vector.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestSATSolverMutableRuntimeClauseSpanMatchesFiniteSlice(t *testing.T) {
	b := arena.NewBudget(^uint64(0))
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b})
	satOK(t, e)
	var c satUintColumn
	c.Bind(r)
	input := []Literal{4, 0, 2, 2, 4}
	for mask := 0; mask < 27; mask++ {
		for scope := uint32(0); scope < 2; scope++ {
			state := satSolverClauseState{assignment: make([]int8, 6), levels: []uint32{0, 1, 0}}
			x := mask
			for v := 0; v < 3; v++ {
				value := lbool.LBool(x%3 - 1)
				x /= 3
				state.assignment[2*v] = int8(value)
				state.assignment[2*v+1] = int8(-value)
			}
			satOK(t, c.Shrink(0))
			for _, l := range input {
				satOK(t, c.Push(uint32(l)))
			}
			native := append([]Literal(nil), input...)
			n, z := uint32(len(input)), uint32(len(input))
			want, e := state.simplifyClause(scope, &n, native)
			satOK(t, e)
			got, e := state.simplifyClauseSpan(scope, &z, satConflictColumnSpan(&c))
			satOK(t, e)
			if got != want || n != z {
				t.Fatal(mask, scope, got, want, n, z)
			}
			for i, w := range native {
				v, e := c.Read(uint32(i))
				satOK(t, e)
				if Literal(v) != w {
					t.Fatal(mask, scope, i, v, w)
				}
			}
		}
	}
	satOK(t, c.Finalize())
	satOK(t, r.Close())
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
}
func TestSATSolverMutableCapturedSpanRejectsClosedColumn(t *testing.T) {
	b := arena.NewBudget(^uint64(0))
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b})
	satOK(t, e)
	var c satUintColumn
	c.Bind(r)
	satOK(t, c.Push(2))
	span := satConflictColumnSpan(&c)
	satOK(t, c.Finalize())
	state := satSolverClauseState{assignment: []int8{0, 0, 0, 0}}
	n := uint32(1)
	_, e = state.simplifyClauseSpan(0, &n, span)
	if e == nil || n != 1 {
		t.Fatal("captured source pointer rebound or count published", e, n)
	}
	if e = span.setLiteral(0, 0); e == nil {
		t.Fatal("actual stale destination write accepted")
	}
	if _, e = state.simplifyClause(0, &n, []Literal{}); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("slice boundary changed", e)
	}
	satOK(t, r.Close())
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
}

func TestClauseConstructorKeepsCopiedPrefixOnShortCapturedColumn(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	runtime, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget})
	satOK(t, err)
	allocator := satStore(t, budget)
	var column satUintColumn
	column.Bind(runtime)
	for _, word := range []uint32{18, 43} {
		satOK(t, column.Push(word))
	}
	clause, err := allocator.mkClauseSpan(3, satConflictColumnSpan(&column), true)
	if !errors.Is(err, arena.ErrBounds) {
		t.Fatalf("short column returned %v", err)
	}
	for i, want := range []Literal{18, 43} {
		got, err := allocator.Literal(clause, uint32(i))
		if err != nil || got != want {
			t.Fatalf("literal %d = %v, %v; want %v", i, got, err, want)
		}
	}
	approx, err := allocator.Approx(clause)
	satOK(t, err)
	learned, err := allocator.Learned(clause)
	satOK(t, err)
	strengthened, err := allocator.Strengthened(clause)
	satOK(t, err)
	if approx != 0 || !learned || strengthened {
		t.Fatal("partial clause header", approx, learned, strengthened)
	}
	satOK(t, allocator.Delete(clause))
	satOK(t, column.Finalize())
	satOK(t, allocator.Close())
	satOK(t, runtime.Close())
	if budget.Used() != 0 {
		t.Fatal(budget.Used())
	}
}
