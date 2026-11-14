package sat

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func satNativeIndexValues(t *testing.T, s *satSolverIndexSet) []uint32 {
	t.Helper()
	it, e := s.capture()
	satOK(t, e)
	var values []uint32
	for {
		v, e := it.next()
		satOK(t, e)
		if !v.present {
			return values
		}
		values = append(values, v.value)
	}
}
func TestSATSolverConsequenceNativeIndexOrderAndDeepMapCopy(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	var set satSolverIndexSet
	defer set.close()
	want := []uint32{17, 1, 9}
	for _, key := range want {
		satOK(t, set.insert(key))
	}
	if got := satNativeIndexValues(t, &set); !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	satOK(t, s.antecedents.insert(7, &set))
	satOK(t, set.reset())
	stored, e := s.antecedents.find(7)
	satOK(t, e)
	if got := satNativeIndexValues(t, stored); !reflect.DeepEqual(got, want) {
		t.Fatal("deep copy", got)
	}
	satOK(t, set.insert(100))
	yes, e := stored.contains(100)
	satOK(t, e)
	if yes {
		t.Fatal("map borrowed caller dictionary")
	}
	satFullSolverRuntimeAudit(t, s)
}

func TestSATSolverConsequenceNativeMapGrowthAndReplacement(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	f, e := s.consequenceFrame()
	satOK(t, e)
	f.indexSet()
	defer func() { satOK(t, s.consequenceDrop(f, nil)) }()
	before := s.budget.Used()
	runtimeBefore := s.runtime.memory.Stats().LiveBlocks
	s.budget.FailAfter(0)
	for k := uint32(0); k < 257; k++ {
		satOK(t, f.index.reset())
		satOK(t, f.index.insert(k))
		satOK(t, f.index.insert(k+1000))
		satOK(t, s.antecedents.insert(k, &f.index))
	}
	for k := uint32(0); k < 257; k++ {
		row, e := s.antecedents.find(k)
		satOK(t, e)
		// 1000 = 0 mod 8: when k sits in the last slot, k+1000 wraps to
		// slot 0, and each map expansion's copy assignment flips the pair.
		got := satNativeIndexValues(t, row)
		if !reflect.DeepEqual(got, []uint32{k, k + 1000}) && !(k%8 == 7 && reflect.DeepEqual(got, []uint32{k + 1000, k})) {
			t.Fatal(k, got)
		}
	}
	row, e := s.antecedents.find(0)
	satOK(t, e)
	it, e := row.capture()
	satOK(t, e)
	satOK(t, f.index.reset())
	satOK(t, f.index.insert(17))
	satOK(t, f.index.insert(29))
	satOK(t, s.antecedents.insert(0, &f.index))
	replaced, e := s.antecedents.find(0)
	satOK(t, e)
	if replaced != row || s.antecedents.size != 257 {
		t.Fatal("replacement changed map identity or size")
	}
	if _, e = it.next(); !errors.Is(e, ErrSATOwner) {
		t.Fatal("assignment retained old iterator", e)
	}
	if got := satNativeIndexValues(t, replaced); !reflect.DeepEqual(got, []uint32{17, 29}) {
		t.Fatal(got)
	}
	satOK(t, s.antecedents.insert(0, replaced)) // Actual self assignment keeps the row.
	if got := satNativeIndexValues(t, replaced); !reflect.DeepEqual(got, []uint32{17, 29}) {
		t.Fatal(got)
	}
	satOK(t, s.antecedents.reset())
	satOK(t, s.antecedents.reset())
	if s.budget.Used() != before || s.runtime.memory.Stats().LiveBlocks != runtimeBefore {
		t.Fatal("native backing charged custom memory")
	}
	satFullSolverRuntimeAudit(t, s)
	s.budget.FailAfter(-1)
}

func TestSATSolverConsequenceNativeSetUnionAssignmentOracle(t *testing.T) {
	var sets [2]satSolverIndexSet
	var membership [2]map[uint32]bool
	membership[0], membership[1] = make(map[uint32]bool), make(map[uint32]bool)
	seed := uint32(0x392a53)
	for step := 0; step < 4096; step++ {
		seed = seed*1664525 + 1013904223
		dst, src := int(seed>>31), int((seed>>30)&1)
		switch seed % 13 {
		case 0:
			satOK(t, sets[dst].reset())
			clear(membership[dst])
		case 1, 2:
			satOK(t, sets[dst].assign(&sets[src]))
			if dst != src {
				clear(membership[dst])
				for k := range membership[src] {
					membership[dst][k] = true
				}
			}
		case 3, 4:
			satOK(t, sets[dst].union(&sets[src]))
			if dst != src {
				for k := range membership[src] {
					membership[dst][k] = true
				}
			}
		default:
			k := (seed >> 8) % 257
			satOK(t, sets[dst].insert(k))
			membership[dst][k] = true
		}
		for i := range sets {
			got := satNativeIndexValues(t, &sets[i])
			if len(got) != len(membership[i]) || uint32(len(got)) != sets[i].size {
				t.Fatal("size oracle", step, i, len(got), len(membership[i]))
			}
			// Slot order: each element sits at its first free probe position
			// from k & mask, so the probe from k & mask to its slot is all used.
			capacity := uint32(len(sets[i].table))
			for slot, entry := range sets[i].table {
				if !entry.used {
					continue
				}
				if !membership[i][entry.key] {
					t.Fatal("membership oracle", step, i, entry.key)
				}
				for j := entry.key & (capacity - 1); j != uint32(slot); j = (j + 1) & (capacity - 1) {
					if !sets[i].table[j].used {
						t.Fatal("probe gap", step, i, entry.key, slot, j)
					}
				}
			}
			for _, k := range []uint32{0, 17, 29, 256, 257, ^uint32(0)} {
				yes, e := sets[i].contains(k)
				satOK(t, e)
				if yes != membership[i][k] {
					t.Fatal("membership oracle", step, i, k, yes)
				}
			}
		}
	}
	for i := range sets {
		satOK(t, sets[i].close())
	}
	if e := sets[0].union(&sets[1]); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed set accepted", e)
	}
}

// AUDIT.md sat_03-F1: index_set is hashtable<unsigned, u_hash, u_eq> and
// iterates in slot order (identity hash, capacity 8, growth at 3/4 load).
func TestSATSolverIndexSetSourceSlotOrder(t *testing.T) {
	build := func(keys ...uint32) *satSolverIndexSet {
		var s satSolverIndexSet
		for _, k := range keys {
			satOK(t, s.insert(k))
		}
		return &s
	}
	want := func(s *satSolverIndexSet, values ...uint32) {
		t.Helper()
		if got := satNativeIndexValues(t, s); !reflect.DeepEqual(got, values) {
			t.Fatal(got, values)
		}
	}
	want(build(6, 4, 2, 8, 16), 8, 16, 2, 4, 6)
	grow := build(0, 2, 4, 6, 8, 10, 12)
	want(grow, 0, 8, 2, 10, 4, 12, 6)
	satOK(t, grow.insert(14))
	want(grow, 0, 2, 4, 6, 8, 10, 12, 14)
	a := build(6)
	b := build(4, 2)
	satOK(t, a.union(b))
	want(a, 2, 4, 6)
	s, _, _ := satSolverCheckFixture(t)
	satOK(t, s.antecedents.insert(7, b))
	stored, e := s.antecedents.find(7)
	satOK(t, e)
	want(stored, 2, 4)
}
