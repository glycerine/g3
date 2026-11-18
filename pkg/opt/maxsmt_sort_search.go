package opt

// Source: src/opt/sortmax.cpp operator() monotone-output search.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *sortMaxEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	status, err := e.preprocessMutexes()
	if err != nil || status != lbool.True {
		return status, false, err
	}
	// sortmax::operator(): offset = m_lower; m_upper = offset; before the
	// expansion that throws for a weight that is not unsigned.
	o := newRatOps(e.solver.manager)
	// rational offset = m_lower; m_upper = offset
	offset := o.zero()
	o.add(&offset, &e.mutexOffset, &e.objective.softOffset)
	defer func() {
		o.del(&offset)
		resultErr = o.joined(resultErr)
	}()
	if !o.ok() {
		return lbool.Undef, false, nil
	}
	if err = e.unscaleTerms(); err != nil {
		return lbool.Undef, false, errors.Join(err, e.setBounds(result, &offset, &offset))
	}
	encoder := newSortMaxEncoder(e)
	e.outputs, err = encoder.network(e.inputs)
	if err != nil {
		return lbool.Undef, false, err
	}
	e.stats.Inputs = uint32(len(e.inputs))
	if err = e.updateResult(result); err != nil {
		return lbool.Undef, false, err
	}
	first, err := e.countSatisfiedInputs(result.Model)
	if err != nil {
		return lbool.Undef, false, err
	}
	lowerValue, upperValue, step := o.zero(), o.zero(), o.int64(int64(e.offset))
	lower, upper := &lowerValue, &upperValue
	defer func() {
		o.del(&step)
		o.del(&upperValue)
		o.del(&lowerValue)
	}()
	o.add(lower, &e.objective.softOffset, &step)
	o.setInt64(&step, int64(len(e.inputs)-first))
	o.add(upper, lower, &step)
	if !o.ok() {
		return lbool.Undef, false, nil
	}
	if err = e.setBounds(result, lower, upper); err != nil {
		return lbool.Undef, false, err
	}
	for i := 0; i < first; i++ {
		if err = e.solver.solver().Assert(e.outputs[i]); err != nil {
			return lbool.Undef, false, err
		}
	}
	for first < len(e.outputs) {
		// trace_bounds("sortmax") (AUDIT.md FUZZ-F5; g3trace builds only).
		if diagnostic.VerboseProgress {
			if err = e.solver.traceBounds("sortmax", e.index, lower, upper); err != nil {
				return lbool.Undef, false, err
			}
		}
		if err = e.solver.solver().Assert(e.outputs[first]); err != nil {
			return lbool.Undef, false, err
		}
		e.stats.Checks++
		status, err = e.solver.solver().CheckSat(nil)
		if err != nil {
			return lbool.Undef, false, err
		}
		// if (m.canceled()) is_sat = l_undef;
		canceled, err := e.owner.manager.Canceled()
		if err != nil {
			return lbool.Undef, false, err
		}
		if canceled {
			status = lbool.Undef
		}
		if status == lbool.Undef {
			return status, false, nil
		}
		if status == lbool.False {
			o.set(lower, upper)
			if !o.ok() {
				return lbool.Undef, false, nil
			}
			return lbool.True, true, e.setBounds(result, lower, upper)
		}
		if err = e.solver.replaceModel(result); err != nil {
			return lbool.Undef, false, err
		}
		if err = e.updateResult(result); err != nil {
			return lbool.Undef, false, err
		}
		e.stats.Improvements++
		first++
		for first < len(e.outputs) {
			isTrue, evalErr := e.outputIsTrue(result.Model, e.outputs[first])
			if evalErr != nil {
				return lbool.Undef, false, evalErr
			}
			if !isTrue {
				break
			}
			if err = e.solver.solver().Assert(e.outputs[first]); err != nil {
				return lbool.Undef, false, err
			}
			first++
		}
		o.setInt64(&step, int64(len(e.outputs)-first))
		o.add(upper, lower, &step)
		if !o.ok() {
			return lbool.Undef, false, nil
		}
		if err = e.setBounds(result, lower, upper); err != nil {
			return lbool.Undef, false, err
		}
	}
	return lbool.True, true, e.setBounds(result, lower, lower)
}
