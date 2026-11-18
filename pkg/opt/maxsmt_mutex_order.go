package opt

// Source: src/opt/maxsmt.cpp maxsmt_compare_soft.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/sorting"
)

// maxSMTSortMutex is process_mutex's std::sort (maxsmt.cpp:195) with
// maxsmt_compare_soft; it goes through the user-approved libstdc++ port
// (maxres_std_sort.go, AUDIT.md opt_01-F3).
func maxSMTSortMutex(q *numeral.RationalManager, mutex []ast.ASTHandle, weights map[ast.ASTHandle]*numeral.Rational) (err error) {
	sorting.LibstdcxxSort(ratWeightDesc{v: mutex, w: weights, cmp: func(a, b *numeral.Rational) int {
		c, e := q.Compare(a, b)
		if e != nil && err == nil {
			err = e
		}
		return c
	}})
	return err
}
