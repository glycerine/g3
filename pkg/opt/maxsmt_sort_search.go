package opt

// Source: src/opt/sortmax.cpp operator() monotone-output search.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *sortMaxEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	status, err := e.preprocessMutexes()
	if err != nil || status != lbool.True {
		return status, false, err
	}
	encoder := newSortMaxEncoder(e)
	e.outputs, err = encoder.network(e.inputs)
	if err != nil {
		return lbool.Undef, false, err
	}
	e.stats.Inputs = uint32(len(e.inputs))
	first, err := e.updateResult(result)
	if err != nil {
		return lbool.Undef, false, err
	}
	for i := 0; i < first; i++ {
		if err = e.solver.backend.Assert(e.outputs[i]); err != nil {
			return lbool.Undef, false, err
		}
	}
	for first < len(e.outputs) {
		if err = e.solver.backend.Assert(e.outputs[first]); err != nil {
			return lbool.Undef, false, err
		}
		e.stats.Checks++
		status, err = e.solver.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, false, err
		}
		if status != lbool.True {
			return status, status == lbool.False, nil
		}
		if err = e.solver.replaceModel(result); err != nil {
			return lbool.Undef, false, err
		}
		next, err := e.updateResult(result)
		if err != nil {
			return lbool.Undef, false, err
		}
		if next <= first {
			return lbool.Undef, false, errors.New("opt: sortmax model did not satisfy asserted output")
		}
		e.stats.Improvements++
		for first++; first < next; first++ {
			if err = e.solver.backend.Assert(e.outputs[first]); err != nil {
				return lbool.Undef, false, err
			}
		}
	}
	return lbool.False, true, nil
}
