package smtcore

// smtArithSetupFacts is the setup routing state the setup tests check,
// read from either arithmetic theory: the replay theory or SMTTheoryArith
// (selected with G3_THEORY_ARITH=port).
type smtArithSetupFacts struct {
	integer, mixed, infinite bool
	propagateEqualities      bool
	propagationThreshold     uint32
	variables                int
}

func smtArithFactsOf(theory SMTTheoryInternalizer) (*smtArithSetupFacts, bool) {
	switch a := theory.(type) {
	case *SMTLinearArithmeticTheory:
		return &smtArithSetupFacts{integer: a.integer, mixed: a.mixed, infinite: a.infinite,
			propagateEqualities: a.propagateEqualities, propagationThreshold: a.propagationThreshold,
			variables: a.NumVariables()}, true
	case *SMTTheoryArith:
		// i_ext is the integer specialization; mi_ext and inf_ext mix.
		return &smtArithSetupFacts{integer: a.integerExt, mixed: !a.integerExt, infinite: a.infExt,
			propagateEqualities: a.params.PropagateEqs, propagationThreshold: a.params.PropagationThreshold,
			variables: a.NumVariables()}, true
	}
	return nil, false
}
