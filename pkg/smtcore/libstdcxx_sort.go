package smtcore

// Source: libstdc++ (GCC 13, the g++ 13.3 that built the z3 4.7.1 fork)
// std::sort with a comparator: bits/stl_algo.h __sort, __introsort_loop,
// __unguarded_partition_pivot, __move_median_to_first,
// __unguarded_partition, __partial_sort/__heap_select (bits/stl_heap.h
// __make_heap, __sort_heap, __pop_heap, __adjust_heap, __push_heap),
// __final_insertion_sort, __insertion_sort, __unguarded_insertion_sort and
// __unguarded_linear_insert.

// =========================================================================
// !!! WARNING: THIS IS A DELIBERATE, USER-APPROVED PORT OF STL INTERNALS !!!
// =========================================================================
//
// The project rule (AGENTS.md) is: never port the C++ standard library.
// This file is one of the few approved exceptions (approved by the user on
// 2026-11-13, alongside the lar_term unordered_map and lu.h unordered_set
// exceptions in pkg/util/lp). It serves exactly TWO approved call sites:
//
//  1. SMTTheoryArith.flushBoundAxioms (smt_theory_arith_core.go), the port
//     of theory_arith<Ext>::flush_bound_axioms in
//     src/smt/theory_arith_core.h (approved 2026-11-13):
//
//       std::sort(atoms.begin(), atoms.end(), compare_atoms());
//       std::sort(occs.begin(), occs.end(), compare_atoms());
//
//     via libstdcxxSortAtoms ([]*smtArithmeticAtom).
//
//  2. SMTTheoryLRA's flush_bound_axioms (theory_lra_bound_axioms.go), the
//     port of theory_lra::imp::flush_bound_axioms in
//     src/smt/theory_lra.cpp:1633-1634 (approved 2026-11-13, extension):
//
//       std::sort(atoms.begin(), atoms.end(), compare_bounds());
//       std::sort(occs.begin(), occs.end(), compare_bounds());
//
//     via libstdcxxSortLRABounds ([]*smtLRABound).
//
// DO NOT replace it with sort.Slice, sort.SliceStable, slices.Sort, or any
// other "equivalent" sort, and do not use it anywhere else without a new
// approval.
//
// WHY IT EXISTS:
//   compare_atoms and compare_bounds compare only the bound value k, so
//   (<= x c) and (>= x c) on the same variable are TIES. std::sort is not
//   stable: with at most 16 elements it is a plain insertion sort (ties keep
//   their input order, which is why a stable sort used to be enough), but
//   with more than 16 atoms introsort's median-of-three partitioning
//   permutes the ties. The sorted order decides which bound axioms
//   (mk_bound_axiom clauses) are created and in what order, which changes
//   clause/literal order and then the whole search. With sort.SliceStable,
//   quantifier fuzz seeds q_2119, q_3063, q_3110 and old_q_515 diverged from
//   the fork in theory_arith, and q_3110 never finished (q_3157's stdout
//   also differed). Reproducing libstdc++'s exact element order fixes all
//   five. theory_lra (smt.arith.solver=6) has the identical pattern.
//
// INDEX/SWAP FORMULATION (why it is still exact):
//   The algorithm works on positions through libstdcxxSortData (Len/Less/
//   Swap, same shape as sort.Interface; an interface, not generics), so one
//   verified implementation serves both element types. libstdc++ moves
//   values: __unguarded_linear_insert, __insertion_sort's move_backward,
//   __pop_heap and __adjust_heap/__push_heap save one value and leave a
//   "hole" that other elements are moved into. Here the saved value instead
//   physically rides along in the hole: each "move a[j] into the hole at h"
//   is Swap(h, j), so the saved value is always at the hole index, and each
//   comparison against the saved value (__comp(__val, __next),
//   __comp(__first + __parent, __value)) becomes Less against the hole
//   index. libstdc++ never compares the stale contents of a hole, so the
//   comparison sequence and the final permutation are identical; the
//   control test below proves it against real libstdc++ output.
//   __iter_swap, used by __move_median_to_first and __unguarded_partition,
//   is a plain Swap.
//
// WHICH TESTS DEPEND ON IT (keep them green):
//   * TestLibstdcxxSortMatchesControl (libstdcxx_sort_test.go) compares this
//     port, through both typed wrappers, against the real libstdc++ output
//     recorded from src/test/stl_sort_order_control.cpp
//     (test_vectors/libstdcxx_sort_order.txt), including tie-heavy inputs
//     of more than 16 elements and adversarial inputs that reach the
//     heap-sort fallback. If it goes red, the port no longer matches z3's
//     std::sort. Fix the port, never the expectations: regenerate the
//     recorded output only by recompiling and rerunning the C++ program.
//   * TestLibstdcxxSortLRABoundsMatchesControl (same file) does the same
//     for *smtLRABound values (theory_lra's compare_bounds).
//   * cmd/g3 vectors flush_bound_axioms_introsort_*.smt2 (theory_arith fuzz
//     seeds whose stdout depends on this order) and
//     lra_flush_bound_axioms_introsort*.smt2 (theory_lra).
//
// Every comparison and move follows the C++ source line by line; keep it
// that way.

import "math/bits"

// libstdcxxSortThreshold is _S_threshold.
const libstdcxxSortThreshold = 16

// libstdcxxSortHeapFallbacks counts __partial_sort fallbacks; the control
// test uses it to prove the heap path was exercised.
var libstdcxxSortHeapFallbacks int

// libstdcxxSortData is the positional view of the sequence being sorted:
// Less(i, j) is __comp(first+i, first+j), Swap(i, j) exchanges two
// positions. Only the two typed wrappers below implement it.
type libstdcxxSortData interface {
	Len() int
	Less(i, j int) bool
	Swap(i, j int)
}

// libstdcxxAtomSlice adapts []*smtArithmeticAtom with compare_atoms.
type libstdcxxAtomSlice struct {
	a    []*smtArithmeticAtom
	less func(x, y *smtArithmeticAtom) bool
}

func (s libstdcxxAtomSlice) Len() int           { return len(s.a) }
func (s libstdcxxAtomSlice) Less(i, j int) bool { return s.less(s.a[i], s.a[j]) }
func (s libstdcxxAtomSlice) Swap(i, j int)      { s.a[i], s.a[j] = s.a[j], s.a[i] }

// libstdcxxSortAtoms is std::sort(a.begin(), a.end(), less) for
// theory_arith's flush_bound_axioms. See the WARNING above.
func libstdcxxSortAtoms(a []*smtArithmeticAtom, less func(x, y *smtArithmeticAtom) bool) {
	libstdcxxSort(libstdcxxAtomSlice{a: a, less: less})
}

// libstdcxxLRABoundsByValue adapts []*smtLRABound with theory_lra's
// compare_bounds: a1->get_value() < a2->get_value() (value only).
type libstdcxxLRABoundsByValue []*smtLRABound

func (s libstdcxxLRABoundsByValue) Len() int           { return len(s) }
func (s libstdcxxLRABoundsByValue) Less(i, j int) bool { return s[i].value.Cmp(&s[j].value) < 0 }
func (s libstdcxxLRABoundsByValue) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// libstdcxxSortLRABounds is std::sort(a.begin(), a.end(), compare_bounds())
// for theory_lra's flush_bound_axioms. See the WARNING above.
func libstdcxxSortLRABounds(a []*smtLRABound) {
	libstdcxxSort(libstdcxxLRABoundsByValue(a))
}

// libstdcxxSort is std::sort(first, last, comp) (__sort).
func libstdcxxSort(a libstdcxxSortData) {
	n := a.Len()
	if n == 0 {
		return
	}
	// __lg(n) * 2
	libstdcxxIntrosortLoop(a, 0, n, 2*(bits.Len(uint(n))-1))
	libstdcxxFinalInsertionSort(a, 0, n)
}

func libstdcxxIntrosortLoop(a libstdcxxSortData, first, last, depthLimit int) {
	for last-first > libstdcxxSortThreshold {
		if depthLimit == 0 {
			// __partial_sort(first, last, last)
			libstdcxxSortHeapFallbacks++
			libstdcxxHeapSelect(a, first, last, last)
			libstdcxxSortHeap(a, first, last)
			return
		}
		depthLimit--
		cut := libstdcxxUnguardedPartitionPivot(a, first, last)
		libstdcxxIntrosortLoop(a, cut, last, depthLimit)
		last = cut
	}
}

func libstdcxxMoveMedianToFirst(a libstdcxxSortData, result, x, y, z int) {
	if a.Less(x, y) {
		if a.Less(y, z) {
			a.Swap(result, y)
		} else if a.Less(x, z) {
			a.Swap(result, z)
		} else {
			a.Swap(result, x)
		}
	} else if a.Less(x, z) {
		a.Swap(result, x)
	} else if a.Less(y, z) {
		a.Swap(result, z)
	} else {
		a.Swap(result, y)
	}
}

func libstdcxxUnguardedPartition(a libstdcxxSortData, first, last, pivot int) int {
	for {
		for a.Less(first, pivot) {
			first++
		}
		last--
		for a.Less(pivot, last) {
			last--
		}
		if !(first < last) {
			return first
		}
		a.Swap(first, last)
		first++
	}
}

func libstdcxxUnguardedPartitionPivot(a libstdcxxSortData, first, last int) int {
	mid := first + (last-first)/2
	libstdcxxMoveMedianToFirst(a, first, first+1, mid, last-1)
	return libstdcxxUnguardedPartition(a, first+1, last, first)
}

func libstdcxxFinalInsertionSort(a libstdcxxSortData, first, last int) {
	if last-first > libstdcxxSortThreshold {
		libstdcxxInsertionSort(a, first, first+libstdcxxSortThreshold)
		// __unguarded_insertion_sort
		for i := first + libstdcxxSortThreshold; i != last; i++ {
			libstdcxxUnguardedLinearInsert(a, i)
		}
	} else {
		libstdcxxInsertionSort(a, first, last)
	}
}

func libstdcxxInsertionSort(a libstdcxxSortData, first, last int) {
	if first == last {
		return
	}
	for i := first + 1; i != last; i++ {
		if a.Less(i, first) {
			// val = move(*i); move_backward(first, i, i+1); *first = val:
			// rotate [first, i] right by one, the saved value riding in
			// the hole.
			for h := i; h > first; h-- {
				a.Swap(h, h-1)
			}
		} else {
			libstdcxxUnguardedLinearInsert(a, i)
		}
	}
}

// libstdcxxUnguardedLinearInsert: val = move(*last) stays at the hole
// index last; __comp(val, next) is Less(last, next).
func libstdcxxUnguardedLinearInsert(a libstdcxxSortData, last int) {
	next := last - 1
	for a.Less(last, next) {
		a.Swap(last, next) // *last = move(*next); the hole moves to next
		last = next
		next--
	}
	// *last = move(val): val is already at last.
}

// libstdcxxHeapSelect is __heap_select(first, middle, last).
func libstdcxxHeapSelect(a libstdcxxSortData, first, middle, last int) {
	libstdcxxMakeHeap(a, first, middle)
	for i := middle; i < last; i++ {
		if a.Less(i, first) {
			libstdcxxPopHeap(a, first, middle, i)
		}
	}
}

func libstdcxxMakeHeap(a libstdcxxSortData, first, last int) {
	if last-first < 2 {
		return
	}
	length := last - first
	parent := (length - 2) / 2
	for {
		// val = move(*(first + parent)): val sits in the hole at parent.
		libstdcxxAdjustHeap(a, first, parent, length)
		if parent == 0 {
			return
		}
		parent--
	}
}

func libstdcxxSortHeap(a libstdcxxSortData, first, last int) {
	for last-first > 1 {
		last--
		libstdcxxPopHeap(a, first, last, last)
	}
}

// libstdcxxPopHeap: val = move(*result); *result = move(*first) is
// Swap(result, first), leaving val in the hole at first (result is always
// outside [first, last), so it is never compared again here).
func libstdcxxPopHeap(a libstdcxxSortData, first, last, result int) {
	a.Swap(result, first)
	libstdcxxAdjustHeap(a, first, 0, last-first)
}

// libstdcxxAdjustHeap is __adjust_heap(first, holeIndex, length, val) with
// val held at first+holeIndex.
func libstdcxxAdjustHeap(a libstdcxxSortData, first, holeIndex, length int) {
	topIndex := holeIndex
	secondChild := holeIndex
	for secondChild < (length-1)/2 {
		secondChild = 2 * (secondChild + 1)
		if a.Less(first+secondChild, first+secondChild-1) {
			secondChild--
		}
		a.Swap(first+holeIndex, first+secondChild)
		holeIndex = secondChild
	}
	if length&1 == 0 && secondChild == (length-2)/2 {
		secondChild = 2 * (secondChild + 1)
		a.Swap(first+holeIndex, first+secondChild-1)
		holeIndex = secondChild - 1
	}
	// __push_heap(first, holeIndex, topIndex, val):
	// __comp(first + parent, val) is Less(parent, hole).
	parent := (holeIndex - 1) / 2
	for holeIndex > topIndex && a.Less(first+parent, first+holeIndex) {
		a.Swap(first+holeIndex, first+parent)
		holeIndex = parent
		parent = (holeIndex - 1) / 2
	}
	// *(first + holeIndex) = move(val): already there.
}
