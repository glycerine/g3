package smtcore

// Source: src/smt/smt_setup.cpp setup_unknown(static_features&).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (k *SMTKernel) withSetupLogic(logic string, action func() error) error {
	previous := k.setupLogic
	k.setupLogic = logic
	defer func() { k.setupLogic = previous }()
	return action()
}

func (k *SMTKernel) setupUnknownStaticPolicy(useStaticFeatures bool) (result error) {
	if !useStaticFeatures || !smtLogicUsesUnknownSetup(k.setupLogic) {
		return nil
	}
	features, err := k.setupStaticFeatures(true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, features.Close()) }()
	if features.NumQuantifiers != 0 {
		// setupQuantifiedLogicPolicy already applies setup_AUFLIA(false) or
		// setup_AUFLIRA(false), including the retained generic plugins.
		return nil
	}

	difference := (features.NumDiffTerms != 0 || features.NumDiffEqualities != 0 || features.NumDiffInequalities != 0) &&
		features.NumArithEqualities == features.NumDiffEqualities &&
		features.NumArithTerms == features.NumDiffTerms &&
		features.NumArithInequalities == features.NumDiffInequalities
	arithmetic := features.NumArithEqualities != 0 || features.NumArithTerms != 0 || features.NumArithInequalities != 0
	theories := features.NumTheories()

	if features.NumNonUFTheories() == 0 {
		return k.withSetupLogic("QF_UF", func() error { return k.setupSimpleLogicPolicy(true) })
	}
	if theories == 1 && difference {
		switch {
		case features.HasReal && !features.HasInt:
			return k.withSetupLogic("QF_RDL", func() error { return k.setupRealDifferencePolicy(true) })
		case !features.HasReal && features.HasInt:
			return k.withSetupLogic("QF_IDL", func() error { return k.setupIntegerDifferencePolicy(true) })
		default:
			return k.setupBasicArithmetic()
		}
	}
	if theories == 2 && features.HasUF() && difference {
		if !features.HasReal && features.HasInt {
			return k.withSetupLogic("QF_UFIDL", func() error { return k.setupUFIntegerDifferencePolicy(true) })
		}
		return k.setupBasicArithmetic()
	}
	if theories == 1 && arithmetic {
		switch {
		case features.HasInt && features.HasReal || features.NumNonLinear != 0:
			return k.installMixedArithmeticSetup(k.smtParams.arith.state)
		case features.HasReal:
			return k.withSetupLogic("QF_LRA", func() error { return k.setupRealLogicPolicy(true) })
		default:
			if err := k.installIntegerArithmeticSetup(k.smtParams.arith.state); err != nil {
				return err
			}
			return k.withSetupLogic("QF_LIA", func() error { return k.setupIntegerLogicPolicy(true) })
		}
	}
	if theories == 2 && features.HasUF() && arithmetic {
		switch {
		case !features.HasReal:
			return k.setupUnknownUFLIA()
		case !features.HasInt && features.NumNonLinear == 0:
			return k.withSetupLogic("QF_UFLRA", func() error { return k.setupSimpleLogicPolicy(true) })
		default:
			return k.setupBasicArithmetic()
		}
	}
	if theories == 1 && features.HasBitVector {
		return k.withSetupLogic("QF_BV", k.setupBitVectorLogicPolicy)
	}
	if theories == 1 && features.HasFloatingPoint {
		return k.withSetupLogic("QF_FP", k.setupBitVectorLogicPolicy)
	}
	if theories == 2 && features.HasFloatingPoint && features.HasBitVector {
		return k.withSetupLogic("QF_FPBV", k.setupBitVectorLogicPolicy)
	}
	if theories == 1 && features.HasArrays {
		return k.withSetupLogic("QF_AX", func() error { return k.setupArrayLogicPolicy(true) })
	}
	if theories == 2 && features.HasUF() && features.HasArrays && features.HasBitVector {
		return k.withSetupLogic("QF_AUFBV", k.setupBitVectorLogicPolicy)
	}
	if theories == 2 && features.HasUF() && features.HasArrays && features.HasInt {
		return k.withSetupLogic("QF_AUFLIA", func() error { return k.setupArrayLogicPolicy(false) })
	}
	return k.setupBasicArithmetic()
}

func (k *SMTKernel) setupUnknownUFLIA() error {
	smt := k.smtParams.state
	arithmetic := k.smtParams.arith.state
	preprocess := k.smtParams.preprocess.state
	smt.RelevancyLvl = 0
	arithmetic.Reflect = false
	arithmetic.PropagationThreshold = 1000
	preprocess.NNFCNF = false
	if err := k.installIntegerArithmeticSetup(arithmetic); err != nil {
		return err
	}
	return k.applyArithmeticSetup(smt, arithmetic, preprocess)
}

func (k *SMTKernel) installIntegerArithmeticSetup(arithmetic SMTArithState) error {
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	theory, err := NewSMTTheoryArith(family, true)
	if err != nil {
		return err
	}
	if err = theory.ConfigureArithmetic(arithmetic); err != nil {
		return err
	}
	if err = k.replaceUnusedSetupArithmetic(theory); err != nil {
		return err
	}
	return nil
}

// Plugin families in the order setup_unknown(st) registers them.
const (
	smtSetupArith = iota
	smtSetupArray
	smtSetupBV
	smtSetupDatatype
	smtSetupDatalog
	smtSetupSeq
	smtSetupPB
	smtSetupFPA
)

// smtUnknownStaticPlugins returns the plugins registered by the profile
// setup_unknown(st) selects, in registration order. setup_unknown() and the
// quantified profile register every plugin; the quantified profile calls
// setup_datatypes before setup_bv.
func smtUnknownStaticPlugins(f *ast.StaticFeatures) []int {
	all := []int{smtSetupArith, smtSetupArray, smtSetupBV, smtSetupDatatype, smtSetupDatalog, smtSetupSeq, smtSetupPB, smtSetupFPA}
	if f.NumQuantifiers != 0 {
		// setup_AUFLIA(false)/setup_AUFLIRA(false), setup_datatypes, setup_bv,
		// setup_dl, setup_seq_str, setup_card, setup_fpa.
		return []int{smtSetupArith, smtSetupArray, smtSetupDatatype, smtSetupBV, smtSetupDatalog, smtSetupSeq, smtSetupPB, smtSetupFPA}
	}
	if f.NumNonUFTheories() == 0 {
		return nil // setup_QF_UF(st)
	}
	n := f.NumTheories()
	difference := (f.NumDiffTerms != 0 || f.NumDiffEqualities != 0 || f.NumDiffInequalities != 0) && f.NumArithEqualities == f.NumDiffEqualities && f.NumArithTerms == f.NumDiffTerms && f.NumArithInequalities == f.NumDiffInequalities
	arithmetic := f.NumArithEqualities != 0 || f.NumArithTerms != 0 || f.NumArithInequalities != 0
	arith := []int{smtSetupArith}
	switch {
	case n == 1 && difference:
		if f.HasReal != f.HasInt {
			return arith // setup_QF_RDL(st) / setup_QF_IDL(st)
		}
		return all
	case n == 2 && f.HasUF() && difference:
		if !f.HasReal && f.HasInt {
			return arith // setup_QF_UFIDL(st)
		}
		return all
	case n == 1 && arithmetic:
		return arith // setup_QF_LIRA(st) / setup_QF_LRA(st) / setup_QF_LIA(st)
	case n == 2 && f.HasUF() && arithmetic:
		if !f.HasReal || !f.HasInt && f.NumNonLinear == 0 {
			return arith // setup_QF_UFLIA(st) / setup_QF_UFLRA()
		}
		return all
	case n == 1 && f.HasBitVector:
		return []int{smtSetupBV} // setup_QF_BV
	case n == 1 && f.HasFloatingPoint, n == 2 && f.HasFloatingPoint && f.HasBitVector:
		return []int{smtSetupBV, smtSetupFPA} // setup_QF_FP / setup_QF_FPBV
	case n == 1 && f.HasArrays:
		return []int{smtSetupArray} // setup_QF_AX(st)
	case n == 2 && f.HasUF() && f.HasArrays && f.HasBitVector:
		return []int{smtSetupBV, smtSetupArray} // setup_QF_AUFBV
	case n == 2 && f.HasUF() && f.HasArrays && f.HasInt:
		return []int{smtSetupArith, smtSetupArray} // setup_QF_AUFLIA
	}
	return all
}

func (k *SMTKernel) smtSetupPluginFamily(plugin int) (ast.FamilyID, error) {
	m := k.manager
	switch plugin {
	case smtSetupArith:
		return m.ArithmeticFamilyID()
	case smtSetupArray:
		return m.ArrayFamilyID()
	case smtSetupBV:
		return m.RegisterBitVectorPlugin()
	case smtSetupDatatype:
		return m.RegisterDatatypePlugin()
	case smtSetupDatalog:
		return m.RegisterDatalogPlugin()
	case smtSetupSeq:
		return m.RegisterSequencePlugin()
	case smtSetupPB:
		return m.RegisterPseudoBooleanPlugin()
	case smtSetupFPA:
		return m.RegisterFloatingPointPlugin()
	}
	return ast.NullFamilyID, ast.ErrASTDomain
}

// setupUnknownStaticPlugins makes the context's plugin set the one
// setup_unknown(st) registers. Go eagerly installs the full setup_unknown()
// plugin set at construction; the source registers only the selected
// profile's plugins at the first check, so unselected placeholders are
// dropped (they would otherwise take final_check rotation slots and run
// their own final checks) and the rest take the profile's order.
func (k *SMTKernel) setupUnknownStaticPlugins(useStaticFeatures bool) (result error) {
	if !useStaticFeatures || !k.smtState.AutoConfig || k.setupPluginsCopied ||
		!smtLogicUsesUnknownSetup(k.setupLogic) || !smtAutoConfigUsesUnknownSetup(k.setupLogic) {
		return nil
	}
	s := k.internalizer
	if len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	features, err := k.setupStaticFeatures(true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, features.Close()) }()
	setupFamilies := make(map[ast.FamilyID]bool, smtSetupFPA+1)
	for plugin := smtSetupArith; plugin <= smtSetupFPA; plugin++ {
		family, err := k.smtSetupPluginFamily(plugin)
		if err != nil {
			return err
		}
		setupFamilies[family] = true
	}
	plugins := smtUnknownStaticPlugins(features)
	order := make([]SMTTheoryInternalizer, 0, len(s.theoryOrder))
	keep := make(map[SMTTheoryInternalizer]bool, len(plugins))
	for _, plugin := range plugins {
		family, err := k.smtSetupPluginFamily(plugin)
		if err != nil {
			return err
		}
		if theory := s.theories[family]; theory != nil && !keep[theory] {
			keep[theory] = true
			order = append(order, theory)
		}
	}
	var dropped []SMTTheoryInternalizer
	for _, theory := range s.theoryOrder {
		if !setupFamilies[theory.FamilyID()] {
			// Plugins registered outside setup keep their relative order.
			order = append(order, theory)
		} else if !keep[theory] {
			dropped = append(dropped, theory)
		}
	}
	s.theoryOrder = order
	for _, theory := range dropped {
		delete(s.theories, theory.FamilyID())
		if theory == k.sequenceSetupPlaceholder {
			k.sequenceSetupPlaceholder = nil
		}
		if closer, ok := theory.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}
