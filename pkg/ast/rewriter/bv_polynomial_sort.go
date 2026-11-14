package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:314,620-622,653-655,855-858
// and src/ast/rewriter/bv_rewriter.cpp:1730,1899 (std::sort calls).

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/sorting"
)

// bvMonomialSlice adapts the argument vector to the approved libstdc++
// std::sort port. mon_lt compares monomials by their variable part, so x
// and (bvmul c x) tie, and with more than 16 arguments libstdc++'s
// introsort permutes ties in a specific way (AUDIT.md ast_rewriter_02-F2).
// The comparator's first domain error is preserved; after an error every
// comparison answers false.
type bvMonomialSlice struct {
	m     *ast.ASTManager
	fid   ast.FamilyID
	order bool
	v     []ast.ASTHandle
	cause error
}

func (s *bvMonomialSlice) Len() int      { return len(s.v) }
func (s *bvMonomialSlice) Swap(i, j int) { s.v[i], s.v[j] = s.v[j], s.v[i] }
func (s *bvMonomialSlice) Less(i, j int) bool {
	if s.cause != nil {
		return false
	}
	less, err := bvMonomialLess(s.m, s.fid, s.order, s.v[i], s.v[j])
	s.cause = err
	return less
}

// bvPolynomialSortMonomials is std::sort(begin, end, lt). It must use the
// user-approved libstdc++ port (pkg/util/sorting/libstdcxx_sort.go), not
// sort.Slice: see the WARNING there.
func bvPolynomialSortMonomials(m *ast.ASTManager, fid ast.FamilyID, order bool, v []ast.ASTHandle) error {
	s := &bvMonomialSlice{m: m, fid: fid, order: order, v: v}
	sorting.LibstdcxxSort(s)
	return s.cause
}
