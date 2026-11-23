package smtcore

// Source: src/smt/smt_setup.cpp setup_unknown and setup_arith.

import "errors"

// The retained arithmetic specializations keep int_only and fixnum false.
// The arithmetic mode selects the backend without inspecting the formulas.
func (k *smtSetup) setupBasicArithmetic() (result error) {
	if k.preserveAllocations {
		features, err := k.setupStaticFeatures(true)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, k.releaseFeatures(features)) }()
	}
	arithmetic := k.smtParams.arith.state
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	var theory SMTTheoryInternalizer
	switch arithmetic.Mode {
	case 0:
		if _, ok := k.internalizer.theories[family].(*SMTDummyTheory); ok {
			return nil
		}
		theory, err = NewSMTDummyTheory(family, "no arithmetic")
	case 1:
		arithmetic.Eq2Ineq = true
		theory, err = NewSMTMixedDiffTheory(family)
	case 3:
		// AS_DENSE_DIFF_LOGIC: fixnum and int_only need m_arith_fixnum and
		// m_arith_int_only, which stay false, so theory_dense_mi.
		arithmetic.Eq2Ineq = true
		theory, err = NewSMTDenseDiffTheory(family, smtDenseMI)
	case 4:
		arithmetic.Eq2Ineq = true
		theory, err = NewSMTUTVPITheory(family)
	case 5:
		theory, err = NewSMTTheoryInfArith(family)
	default:
		if existing, ok := k.internalizer.theories[family].(*SMTTheoryArith); ok {
			// Direct internalizer clients may have already created variables.
			// Their concrete specialization must remain stable thereafter.
			if existing.NumVariables() == 0 && !k.copiedArithmeticPlugin {
				existing.useMixedExt()
			}
			// theory_arith reads the context's theory_arith_params by
			// reference, so setup-time changes (pdr's eager_eq_axioms,
			// bound_prop, ...) reach an already registered plugin.
			// (m_random and m_eager_gcd were fixed by the constructor.) A
			// copied plugin keeps its source's parameters (mk_fresh).
			if !k.copiedArithmeticPlugin {
				existing.params = arithmetic
			}
			return nil
		}
		theory, err = NewSMTTheoryArith(family, false)
	}
	if err != nil {
		return err
	}
	if configurable, ok := theory.(interface{ ConfigureArithmetic(SMTArithState) error }); ok {
		if err := configurable.ConfigureArithmetic(arithmetic); err != nil {
			return err
		}
	}
	if err := k.replaceUnusedSetupArithmetic(theory); err != nil {
		return err
	}
	if arithmetic.Mode == 1 || arithmetic.Mode == 3 || arithmetic.Mode == 4 {
		return k.applyArithmeticSetup(k.smtParams.state, arithmetic, k.smtParams.preprocess.state)
	}
	return nil
}
