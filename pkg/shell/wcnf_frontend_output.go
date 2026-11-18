package shell

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/numeral"
	"strconv"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *WCNFFrontend) WriteResult() (resultErr error) {
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
		if e := f.context.WriteOptimizationFrontendStatistics(f.output, f.elapsed.Seconds()); e != nil {
			return e
		}
	}
	valuesAvailable := f.status == lbool.True
	if f.status == lbool.Undef && f.summary.SoftClauses > 0 {
		_, err := f.context.ObjectiveBound(0, true)
		valuesAvailable = err == nil
	}
	if valuesAvailable && f.summary.SoftClauses > 0 {
		if e := f.context.WriteOptimizationFrontendValues(f.output); e != nil {
			return e
		}
	}
	if !valuesAvailable && f.summary.SoftClauses > 0 {
		// The soft weights' sum, a rational of the context's rational
		// manager.
		q := f.context.NumeralService().Manager
		upper, weight := numeral.NewRational(), numeral.NewRational()
		defer func() { resultErr = errors.Join(resultErr, q.Del(&upper), q.Del(&weight)) }()
		for _, clause := range f.document.Clauses {
			if clause.Weight < f.document.TopWeight {
				if e := errors.Join(q.SetString(&weight, strconv.FormatUint(clause.Weight, 10)), q.Add(&upper, &weight, &upper)); e != nil {
					return e
				}
			}
		}
		if upper.IsZero() {
			if _, e := fmt.Fprintln(f.output, "   0"); e != nil {
				return e
			}
		} else {
			text, e := q.String(&upper)
			if e != nil {
				return e
			}
			if _, e = fmt.Fprintf(f.output, "  [0:%s]\n", text); e != nil {
				return e
			}
		}
	}
	return nil
}

func (f *WCNFFrontend) WriteProgress() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrWCNFFrontend
	}
	if !f.initialized {
		return nil
	}
	if f.options.DisplayStatistics {
		if err := f.context.WriteOptimizationFrontendStatistics(f.output, f.elapsed.Seconds()); err != nil {
			return err
		}
	}
	return f.context.WriteOptimizationFrontendValues(f.output)
}
