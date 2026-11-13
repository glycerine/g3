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
// exceptions in pkg/util/lp). It serves exactly ONE call site:
// SMTTheoryArith.flushBoundAxioms (smt_theory_arith_core.go), the port of
// theory_arith<Ext>::flush_bound_axioms in src/smt/theory_arith_core.h:
//
//       std::sort(atoms.begin(), atoms.end(), compare_atoms());
//       std::sort(occs.begin(), occs.end(), compare_atoms());
//
// DO NOT replace it with sort.Slice, sort.SliceStable, slices.Sort, or any
// other "equivalent" sort, and do not use it anywhere else without a new
// approval.
//
// WHY IT EXISTS:
//   compare_atoms compares only the bound value k, so (<= x c) and (>= x c)
//   on the same variable are TIES. std::sort is not stable: with at most 16
//   elements it is a plain insertion sort (ties keep their input order,
//   which is why a stable sort used to be enough), but with more than 16
//   atoms introsort's median-of-three partitioning permutes the ties. The
//   sorted order decides which bound axioms (mk_bound_axiom clauses) are
//   created and in what order, which changes clause/literal order and then
//   the whole search. With sort.SliceStable, quantifier fuzz seeds q_2119,
//   q_3063, q_3110 and old_q_515 diverged from the fork, and q_3110 never
//   finished (q_3157's stdout also differed). Reproducing libstdc++'s exact
//   element moves fixes all five.
//
// WHICH TESTS DEPEND ON IT (keep them green):
//   * TestLibstdcxxSortMatchesControl (libstdcxx_sort_test.go) compares this
//     port against the real libstdc++ output recorded from
//     src/test/stl_sort_order_control.cpp
//     (test_vectors/libstdcxx_sort_order.txt), including tie-heavy inputs
//     of more than 16 elements and adversarial inputs that reach the
//     heap-sort fallback. If it goes red, the port no longer matches z3's
//     std::sort. Fix the port, never the expectations: regenerate the
//     recorded output only by recompiling and rerunning the C++ program.
//   * cmd/g3 vectors flush_bound_axioms_introsort_*.smt2 (fuzz seeds whose
//     stdout depends on this order).
//
// The port is written for the concrete element type of the one call site
// (no generics, no interface adapter). Every comparison and move follows the
// C++ source line by line; keep it that way.

import "math/bits"

// libstdcxxSortThreshold is _S_threshold.
const libstdcxxSortThreshold = 16

// libstdcxxSortHeapFallbacks counts __partial_sort fallbacks; the control
// test uses it to prove the heap path was exercised.
var libstdcxxSortHeapFallbacks int

// libstdcxxSortAtoms is std::sort(a.begin(), a.end(), less) (__sort).
// See the WARNING above.
func libstdcxxSortAtoms(a []*smtArithmeticAtom, less func(x, y *smtArithmeticAtom) bool) {
	n := len(a)
	if n == 0 {
		return
	}
	// __lg(n) * 2
	libstdcxxIntrosortLoop(a, 0, n, 2*(bits.Len(uint(n))-1), less)
	libstdcxxFinalInsertionSort(a, 0, n, less)
}

func libstdcxxIntrosortLoop(a []*smtArithmeticAtom, first, last, depthLimit int, less func(x, y *smtArithmeticAtom) bool) {
	for last-first > libstdcxxSortThreshold {
		if depthLimit == 0 {
			// __partial_sort(first, last, last)
			libstdcxxSortHeapFallbacks++
			libstdcxxHeapSelect(a, first, last, last, less)
			libstdcxxSortHeap(a, first, last, less)
			return
		}
		depthLimit--
		cut := libstdcxxUnguardedPartitionPivot(a, first, last, less)
		libstdcxxIntrosortLoop(a, cut, last, depthLimit, less)
		last = cut
	}
}

func libstdcxxMoveMedianToFirst(a []*smtArithmeticAtom, result, x, y, z int, less func(x, y *smtArithmeticAtom) bool) {
	if less(a[x], a[y]) {
		if less(a[y], a[z]) {
			a[result], a[y] = a[y], a[result]
		} else if less(a[x], a[z]) {
			a[result], a[z] = a[z], a[result]
		} else {
			a[result], a[x] = a[x], a[result]
		}
	} else if less(a[x], a[z]) {
		a[result], a[x] = a[x], a[result]
	} else if less(a[y], a[z]) {
		a[result], a[z] = a[z], a[result]
	} else {
		a[result], a[y] = a[y], a[result]
	}
}

func libstdcxxUnguardedPartition(a []*smtArithmeticAtom, first, last, pivot int, less func(x, y *smtArithmeticAtom) bool) int {
	for {
		for less(a[first], a[pivot]) {
			first++
		}
		last--
		for less(a[pivot], a[last]) {
			last--
		}
		if !(first < last) {
			return first
		}
		a[first], a[last] = a[last], a[first]
		first++
	}
}

func libstdcxxUnguardedPartitionPivot(a []*smtArithmeticAtom, first, last int, less func(x, y *smtArithmeticAtom) bool) int {
	mid := first + (last-first)/2
	libstdcxxMoveMedianToFirst(a, first, first+1, mid, last-1, less)
	return libstdcxxUnguardedPartition(a, first+1, last, first, less)
}

func libstdcxxFinalInsertionSort(a []*smtArithmeticAtom, first, last int, less func(x, y *smtArithmeticAtom) bool) {
	if last-first > libstdcxxSortThreshold {
		libstdcxxInsertionSort(a, first, first+libstdcxxSortThreshold, less)
		// __unguarded_insertion_sort
		for i := first + libstdcxxSortThreshold; i != last; i++ {
			libstdcxxUnguardedLinearInsert(a, i, less)
		}
	} else {
		libstdcxxInsertionSort(a, first, last, less)
	}
}

func libstdcxxInsertionSort(a []*smtArithmeticAtom, first, last int, less func(x, y *smtArithmeticAtom) bool) {
	if first == last {
		return
	}
	for i := first + 1; i != last; i++ {
		if less(a[i], a[first]) {
			val := a[i]
			copy(a[first+1:i+1], a[first:i]) // move_backward(first, i, i+1)
			a[first] = val
		} else {
			libstdcxxUnguardedLinearInsert(a, i, less)
		}
	}
}

func libstdcxxUnguardedLinearInsert(a []*smtArithmeticAtom, last int, less func(x, y *smtArithmeticAtom) bool) {
	val := a[last]
	next := last - 1
	for less(val, a[next]) {
		a[last] = a[next]
		last = next
		next--
	}
	a[last] = val
}

// libstdcxxHeapSelect is __heap_select(first, middle, last).
func libstdcxxHeapSelect(a []*smtArithmeticAtom, first, middle, last int, less func(x, y *smtArithmeticAtom) bool) {
	libstdcxxMakeHeap(a, first, middle, less)
	for i := middle; i < last; i++ {
		if less(a[i], a[first]) {
			libstdcxxPopHeap(a, first, middle, i, less)
		}
	}
}

func libstdcxxMakeHeap(a []*smtArithmeticAtom, first, last int, less func(x, y *smtArithmeticAtom) bool) {
	if last-first < 2 {
		return
	}
	length := last - first
	parent := (length - 2) / 2
	for {
		val := a[first+parent]
		libstdcxxAdjustHeap(a, first, parent, length, val, less)
		if parent == 0 {
			return
		}
		parent--
	}
}

func libstdcxxSortHeap(a []*smtArithmeticAtom, first, last int, less func(x, y *smtArithmeticAtom) bool) {
	for last-first > 1 {
		last--
		libstdcxxPopHeap(a, first, last, last, less)
	}
}

func libstdcxxPopHeap(a []*smtArithmeticAtom, first, last, result int, less func(x, y *smtArithmeticAtom) bool) {
	val := a[result]
	a[result] = a[first]
	libstdcxxAdjustHeap(a, first, 0, last-first, val, less)
}

func libstdcxxAdjustHeap(a []*smtArithmeticAtom, first, holeIndex, length int, val *smtArithmeticAtom, less func(x, y *smtArithmeticAtom) bool) {
	topIndex := holeIndex
	secondChild := holeIndex
	for secondChild < (length-1)/2 {
		secondChild = 2 * (secondChild + 1)
		if less(a[first+secondChild], a[first+secondChild-1]) {
			secondChild--
		}
		a[first+holeIndex] = a[first+secondChild]
		holeIndex = secondChild
	}
	if length&1 == 0 && secondChild == (length-2)/2 {
		secondChild = 2 * (secondChild + 1)
		a[first+holeIndex] = a[first+secondChild-1]
		holeIndex = secondChild - 1
	}
	// __push_heap(first, holeIndex, topIndex, val)
	parent := (holeIndex - 1) / 2
	for holeIndex > topIndex && less(a[first+parent], val) {
		a[first+holeIndex] = a[first+parent]
		holeIndex = parent
		parent = (holeIndex - 1) / 2
	}
	a[first+holeIndex] = val
}
