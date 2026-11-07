package smtcore

// smtArithSetupFacts is the setup routing state the setup tests check,
// read from SMTTheoryArith.
type smtArithSetupFacts struct {
	integer, mixed, infinite bool
	propagateEqualities      bool
	propagationThreshold     uint32
	variables                int
}

func smtArithFactsOf(theory SMTTheoryInternalizer) (*smtArithSetupFacts, bool) {
	switch a := theory.(type) {
	case *SMTTheoryArith:
		// i_ext is the integer specialization; mi_ext and inf_ext mix.
		return &smtArithSetupFacts{integer: a.integerExt, mixed: !a.integerExt, infinite: a.infExt,
			propagateEqualities: a.params.PropagateEqs, propagationThreshold: a.params.PropagationThreshold,
			variables: a.NumVariables()}, true
	}
	return nil, false
}
