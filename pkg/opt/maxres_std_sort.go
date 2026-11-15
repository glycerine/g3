package opt

// Source: src/opt/maxres.cpp:413-424 (sort_assumptions, compare_asm) and
// src/opt/maxsmt.cpp:177-195 (process_mutex, maxsmt_compare_soft).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// =========================================================================
// !!! USER-APPROVED libstdc++ std::sort CALL SITES (owner, 2026-11-14) !!!
// =========================================================================
//
// AUDIT.md opt_01-F3. maxres::sort_assumptions (maxres.cpp:417) and
// maxsmt_solver_base::process_mutex (maxsmt.cpp:195) order expressions with
// std::sort and a weight-only "greater" comparator, so equal weights tie.
// With more than 16 expressions libstdc++'s introsort permutes the ties,
// and that order is the assumption order handed to check_sat (and the mutex
// weight order), which shapes cores and models. These wrappers must use
// pkg/util/sorting/libstdcxx_sort.go (sorting.LibstdcxxSort), never sort.Slice/sort.SliceStable.

// ratWeightDesc is compare_asm / maxsmt_compare_soft over rational weights.
type ratWeightDesc struct {
	v []ast.ASTHandle
	w map[ast.ASTHandle]*big.Rat
}

func (s ratWeightDesc) Len() int           { return len(s.v) }
func (s ratWeightDesc) Swap(i, j int)      { s.v[i], s.v[j] = s.v[j], s.v[i] }
func (s ratWeightDesc) Less(i, j int) bool { return s.w[s.v[i]].Cmp(s.w[s.v[j]]) > 0 }

// intWeightDesc is maxsmt_compare_soft over the integer weights of the
// maxsmt mutex decomposition.
type intWeightDesc struct {
	v []ast.ASTHandle
	w map[ast.ASTHandle]*big.Int
}

func (s intWeightDesc) Len() int           { return len(s.v) }
func (s intWeightDesc) Swap(i, j int)      { s.v[i], s.v[j] = s.v[j], s.v[i] }
func (s intWeightDesc) Less(i, j int) bool { return s.w[s.v[i]].Cmp(s.w[s.v[j]]) > 0 }
