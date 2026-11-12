package lp

// Source: libstdc++ (GCC 13) std::unordered_set<unsigned> with
// std::hash<unsigned> (identity), _Mod_range_hashing and
// _Prime_rehash_policy, unique keys: the same _Hashtable as the
// std::unordered_map<unsigned, V> in std_unordered_map.go, without values.

// =========================================================================
// !!! WARNING: THIS IS A DELIBERATE, USER-APPROVED PORT OF STL INTERNALS !!!
// =========================================================================
//
// The project rule (AGENTS.md) is: never port the C++ standard library.
// This file is one of the few approved exceptions (approved by the user on
// 2026-11-12, alongside the lar_term unordered_map exception of 2026-11-10).
// DO NOT "simplify" it into a Go map, a sorted slice, or insdict.
//
// WHY IT EXISTS:
//   z3's lu.h get_set_of_columns_to_replace_for_add_last_rows() collects the
//   basis columns to replace into a std::unordered_set<unsigned>, and
//   add_last_rows_to_B() then calls replace_column() on them IN THAT SET'S
//   ITERATION ORDER, stopping early if the factorization degenerates:
//
//       for (unsigned j : columns_to_replace) {
//           replace_column_with_only_change_at_last_rows(j, heading[j]);
//           if (get_status() == LU_status::Degenerated) break;
//       }
//
//   Each replace_column() updates the LU factorization, so a different order
//   yields a numerically different factorization, can degenerate at a
//   different point, and can then change later pivots. libstdc++'s order is
//   NOT ascending (e.g. inserting 0,1,2 iterates 2 1 0), and a Go map's order
//   is random, so neither reproduces z3. To stay faithful to z3 4.7.1 we
//   reproduce libstdc++'s exact node-list order (bucket layout,
//   insert-at-bucket-begin, prime rehash policy) by reusing the
//   StdUnorderedRationalMap emulation.
//
// WHICH TESTS DEPEND ON IT (keep them green):
//   * TestStdUnorderedUintSetMatchesLibstdcxx (std_unordered_set_test.go)
//     compares this type against the real libstdc++ output recorded from
//     src/test/stl_unordered_set_order_control.cpp
//     (test_vectors/libstdcxx_unordered_set_order.txt). If it goes red, this
//     emulation no longer matches z3's container and lu_extend diverges from
//     z3. Do not "fix" the test by editing the recorded output; regenerate it
//     only by recompiling and rerunning the C++ control program.
//   * The lu_extend tests (lu_extend_test.go, lu_extend_failure_test.go)
//     exercise AddLastRowsToB through this type.
//   * cmd/g3 vector lra_lu_extend_add_last_rows.smt2
//     (smt.arith.solver 6, lp.simplex_strategy 2) drives the real
//     lar_solver add_last_rows_to_lu path end to end.
//   Honest note (2026-11-12): on the vectors and 60 random solver-6 probes
//   checked so far, ascending order happened to give the same stdout and
//   trace=true log as this exact order, because the LP internals are not
//   traced; the positive control above is what proves fidelity. Keep the
//   exact order anyway: any input where the replacement order changes the
//   factorization or the degenerate break would otherwise diverge silently.

// StdUnorderedUintSet is std::unordered_set<unsigned>: iteration follows
// libstdc++'s node list, so insertion and rehash place keys exactly as the
// C++ container does. See the WARNING above before changing anything.
type StdUnorderedUintSet struct {
	table *StdUnorderedRationalMap
}

// NewStdUnorderedUintSet is a default-constructed set (one bucket).
func NewStdUnorderedUintSet() *StdUnorderedUintSet {
	return &StdUnorderedUintSet{table: NewStdUnorderedRationalMap()}
}

// Insert is insert(key); an existing key keeps its node.
func (s *StdUnorderedUintSet) Insert(key uint32) {
	if _, ok := s.table.Get2(key); ok {
		return
	}
	s.table.Put(key, nil)
}

// Contains is count(key) != 0.
func (s *StdUnorderedUintSet) Contains(key uint32) bool {
	_, ok := s.table.Get2(key)
	return ok
}

// Len is size().
func (s *StdUnorderedUintSet) Len() int { return s.table.Len() }

// Keys lists the keys in iteration (begin() to end()) order. This order is
// the whole point of the type: callers must iterate Keys(), never re-sort.
func (s *StdUnorderedUintSet) Keys() []uint32 { return s.table.Keys() }
