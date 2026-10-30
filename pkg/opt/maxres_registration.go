package opt

// Source: src/opt/maxsmt.cpp MaxRes engine dispatch. This registration lands
// only after the complete core-guided engine is present.

func init() {
	maxSMTFallbackFactory = func(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (maxSMTEngine, error) {
		return newMaxResEngine(s, objective, index, config)
	}
}
