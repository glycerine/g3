package opt

// Source: src/opt/maxsmt.cpp maxsat_engine dispatch.

type maxSMTEngineFactory func(*ArithmeticSolver, *arithmeticObjective, int, maxSMTConfig) (maxSMTEngine, error)

var maxSMTFallbackFactory maxSMTEngineFactory

func selectMaxSMTEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (maxSMTEngine, error) {
	if config.engine == "sortmax" {
		engine, usable, err := newSortMaxEngine(s, objective, index)
		if err != nil || usable {
			return engine, err
		}
	}
	if maxSMTFallbackFactory != nil {
		return maxSMTFallbackFactory(s, objective, index, config)
	}
	return nil, nil
}
