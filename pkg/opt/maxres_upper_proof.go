package opt

// Source: src/opt/maxres.cpp upper/lower termination after an upper block.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) upperBoundProvesOptimal() (bool, error) {
	if !e.upperBoundActive || !e.bestSet {
		return false, nil
	}
	e.stats.Checks++
	status, err := e.solver.backend.CheckSat(nil)
	if err != nil {
		return false, err
	}
	if status == lbool.Undef {
		return false, errMaxResState
	}
	if status == lbool.False {
		e.stats.UpperBoundProofs++
		return true, nil
	}
	return false, nil
}
