package shell

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *OPBFrontend) WriteResult() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrOPBFrontend
	}
	status := "unknown"
	if f.status == lbool.True {
		status = "sat"
	} else if f.status == lbool.False {
		status = "unsat"
	}
	if _, err := fmt.Fprintln(f.output, status); err != nil {
		return err
	}
	if f.options.DisplayStatistics {
		if err := f.context.WriteStatisticsWithTotalTime(f.output, f.elapsed.Seconds()); err != nil {
			return err
		}
	}
	if f.status == lbool.True && f.summary.Objectives != 0 {
		if err := f.context.WriteOptimizationFrontendValues(f.output); err != nil {
			return err
		}
	}
	return nil
}
