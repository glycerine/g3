package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:555-566 (lt_proc,
// sort_roots) and :1887-1889, :2959-2961 (lt).

import "github.com/glycerine/g3/pkg/util/sorting"

// =========================================================================
// !!! USER-APPROVED libstdc++ std::sort CALL SITE (owner, 2026-11-14) !!!
// =========================================================================
//
// AUDIT.md math_polynomial_01-F1. imp::sort_roots is
//
//	std::sort(r.begin(), r.end(), lt_proc(m_wrapper));
//
// (algebraic_numbers.cpp:564), and every comparison is manager::lt, i.e.
// imp::compare, which traces "comparing:" and refines the isolating
// intervals of both arguments in place. Which pairs are compared, in which
// order and with which argument first therefore shows up in the trace and
// in the stored intervals of the returned roots. It must use
// pkg/util/sorting/libstdcxx_sort.go, never a Go sort or a heap sort.

// algebraicRootsByValue is lt_proc over the output vector. Swapping two
// positions moves the cell handles, as the C++ moves anum::m_cell. The
// first Compare error is kept; after it every comparison answers false
// without comparing (the C++ comparison would have thrown out of the sort).
type algebraicRootsByValue struct {
	m     *AlgebraicRootCore
	v     *AlgebraicVector
	cause error
}

func (s *algebraicRootsByValue) Len() int { return len(s.v.values) }
func (s *algebraicRootsByValue) Swap(i, j int) {
	s.v.values[i], s.v.values[j] = s.v.values[j], s.v.values[i]
}
func (s *algebraicRootsByValue) Less(i, j int) bool {
	if s.cause != nil {
		return false
	}
	d, err := s.m.Compare(&s.v.values[i], &s.v.values[j])
	if err != nil {
		s.cause = err
		return false
	}
	return d < 0
}

// sortRoots is imp::sort_roots.
func (m *AlgebraicRootCore) sortRoots(out *AlgebraicVector) error {
	s := &algebraicRootsByValue{m: m, v: out}
	sorting.LibstdcxxSort(s)
	return s.cause
}
