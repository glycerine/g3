package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:32-56
// (sort_non_basis_rational) and :60-79 (sort_non_basis), both std::sort.

import "github.com/glycerine/g3/pkg/util/sorting"

// =========================================================================
// !!! USER-APPROVED libstdc++ std::sort CALL SITE (owner, 2026-11-14) !!!
// =========================================================================
//
// AUDIT.md util_lp_05-F1. lp_primal_core_solver::sort_non_basis_rational
// (lp_primal_core_solver.hpp:34 and :41) and sort_non_basis (:68) order
// m_nbasis with std::sort. The column-count comparator ties constantly and
// treats a zero count as neither before nor after a nonzero one, so the
// order (and with it the entering-column scan of choose_entering_column and
// the lp RNG stream) depends on libstdc++'s exact algorithm. These wrappers
// must use pkg/util/sorting/libstdcxx_sort.go, never sort.Slice.

// nbasisByColumnCount is the m_nbasis view sorted by sort_non_basis_rational:
// counts[j] is number_of_non_zeroes_in_column(j) (tableau) or
// m_columns_nz[j].
type nbasisByColumnCount struct {
	s     []uint32
	count func(j uint32) uint32
}

func (n nbasisByColumnCount) Len() int      { return len(n.s) }
func (n nbasisByColumnCount) Swap(i, j int) { n.s[i], n.s[j] = n.s[j], n.s[i] }
func (n nbasisByColumnCount) Less(i, j int) bool {
	ca, cb := n.count(n.s[i]), n.count(n.s[j])
	if ca == 0 && cb != 0 {
		return false
	}
	return ca < cb
}

// sortNonBasisByColumnCount is the std::sort of sort_non_basis_rational
// (lp_primal_core_solver.hpp:34 tableau, :41 otherwise).
func sortNonBasisByColumnCount(nbasis []uint32, count func(j uint32) uint32) {
	sorting.LibstdcxxSort(nbasisByColumnCount{s: nbasis, count: count})
}

// nbasisBySteepestEdge is sort_non_basis's comparator:
// m_steepest_edge_coefficients[a] > m_steepest_edge_coefficients[b].
type nbasisBySteepestEdge struct {
	s     []uint32
	coeff []float64
}

func (n nbasisBySteepestEdge) Len() int      { return len(n.s) }
func (n nbasisBySteepestEdge) Swap(i, j int) { n.s[i], n.s[j] = n.s[j], n.s[i] }
func (n nbasisBySteepestEdge) Less(i, j int) bool {
	return n.coeff[n.s[i]] > n.coeff[n.s[j]]
}

// sortNonBasisBySteepestEdge is the std::sort of sort_non_basis
// (lp_primal_core_solver.hpp:68).
func sortNonBasisBySteepestEdge(nbasis []uint32, coeff []float64) {
	sorting.LibstdcxxSort(nbasisBySteepestEdge{s: nbasis, coeff: coeff})
}
