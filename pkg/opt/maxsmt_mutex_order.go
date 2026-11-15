package opt

// Source: src/opt/maxsmt.cpp maxsmt_compare_soft.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/sorting"
)

// maxSMTSortMutex is process_mutex's std::sort (maxsmt.cpp:195) with
// maxsmt_compare_soft; it goes through the user-approved libstdc++ port
// (maxres_std_sort.go, AUDIT.md opt_01-F3).
func maxSMTSortMutex(mutex []ast.ASTHandle, weights map[ast.ASTHandle]*big.Int) {
	sorting.LibstdcxxSort(intWeightDesc{v: mutex, w: weights})
}
