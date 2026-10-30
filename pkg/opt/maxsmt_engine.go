package opt

// Source: src/opt/maxsmt.h maxsmt_solver and src/opt/maxsmt.cpp engine
// dispatch retained by ArithmeticSolver.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

type maxSMTEngine interface {
	optimize(*ArithmeticResult) (lbool.LBool, bool, error)
	statistics() MaxSMTStatistics
	close() error
}

func newMaxSMTEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (maxSMTEngine, error) {
	config, err := loadMaxSMTConfig(s)
	if err != nil {
		return nil, err
	}
	return selectMaxSMTEngine(s, objective, index, config)
}
