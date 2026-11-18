package opt

// Source: src/opt/maxsmt.cpp maxsmt_compare_soft for rational weights.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/sorting"
)

// sortMaxResMutex is process_mutex's std::sort (maxsmt.cpp:195) with
// maxsmt_compare_soft; it goes through the user-approved libstdc++ port
// (maxres_std_sort.go, AUDIT.md opt_01-F3).
func (e *maxResEngine) sortMaxResMutex(mutex []ast.ASTHandle, weights map[ast.ASTHandle]*numeral.Rational) {
	sorting.LibstdcxxSort(ratWeightDesc{v: mutex, w: weights, cmp: e.cmp})
}
