package opt

// Source: src/opt/maxres.cpp mus_solver/primal_dual_solver loop check.

import "github.com/glycerine/g3/pkg/util/lbool"

// checkHillClimb is check_sat_hill_climb(m_asms): it reports l_true only
// once every assumption holds.
func (e *maxResEngine) checkHillClimb(result *ArithmeticResult) (status lbool.LBool, full bool, err error) {
	status, err = e.checkSatHillClimb(e.active, result)
	return status, status == lbool.True, err
}
