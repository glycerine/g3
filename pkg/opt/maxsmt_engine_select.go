package opt

// Source: src/opt/maxsmt.cpp maxsat_engine dispatch.

import "github.com/glycerine/g3/pkg/util/diagnostic"

type maxSMTEngineFactory func(*ArithmeticSolver, *arithmeticObjective, int, maxSMTConfig) (maxSMTEngine, error)

var maxSMTFallbackFactory maxSMTEngineFactory

func sourceMaxSMTEngine(name string) (string, error) {
	switch name {
	case "", "maxres", "pd-maxres", "wmax", "sortmax":
		return name, nil
	default:
		// maxsmt::operator(): warning_msg("solver %s is not recognized,
		// using default 'maxres'", ...), then mk_maxres.
		return "maxres", diagnostic.BorrowProcessWarningService().Warningf("solver %s is not recognized, using default 'maxres'", name)
	}
}

func selectMaxSMTEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (maxSMTEngine, error) {
	engine, err := sourceMaxSMTEngine(config.engine)
	if err != nil {
		return nil, err
	}
	config.engine = engine
	if config.engine == "wmax" {
		return newWMaxEngine(s, objective, index)
	}
	if config.engine == "sortmax" {
		engine, err := newSortMaxEngine(s, objective, index)
		if err != nil {
			return nil, err
		}
		return engine, nil
	}
	if maxSMTFallbackFactory != nil {
		return maxSMTFallbackFactory(s, objective, index, config)
	}
	return nil, nil
}
