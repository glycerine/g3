package shell

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
)

func (f *WCNFFrontend) WriteResult() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrWCNFFrontend
	}
	s := "unknown"
	if f.status == lbool.True {
		s = "sat"
	} else if f.status == lbool.False {
		s = "unsat"
	}
	if _, e := fmt.Fprintln(f.output, s); e != nil {
		return e
	}
	if f.options.DisplayStatistics {
		if e := f.context.WriteStatisticsWithTotalTime(f.output, f.elapsed.Seconds()); e != nil {
			return e
		}
	}
	if f.status == lbool.True && f.summary.SoftClauses > 0 {
		if e := f.context.WriteOptimizationFrontendValues(f.output); e != nil {
			return e
		}
	}
	if f.status == lbool.False && f.summary.SoftClauses > 0 {
		var upper big.Int
		for _, clause := range f.document.Clauses {
			if clause.Weight < f.document.TopWeight {
				upper.Add(&upper, new(big.Int).SetUint64(clause.Weight))
			}
		}
		if _, e := fmt.Fprintf(f.output, "  [0:%s]\n", upper.String()); e != nil {
			return e
		}
	}
	return nil
}
