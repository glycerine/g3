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
	if !useStaticFeatures || !smtAutoConfigUsesUnknownSetup(k.setupLogic) {
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

// smtSetupAllPlugins is setup_unknown()'s registration order.
func smtSetupAllPlugins() []int {
	return []int{smtSetupArith, smtSetupArray, smtSetupBV, smtSetupDatatype, smtSetupDatalog, smtSetupSeq, smtSetupPB, smtSetupFPA}
}

// recordPresetupPlugin notes a plugin registered with
// context::register_plugin before setup (copy_plugins, opt_solver::ensure_pb):
// it precedes every plugin the setup profile adds.
func (k *SMTKernel) recordPresetupPlugin(family ast.FamilyID) {
	if k.preprocessingSetupDone {
		return
	}
	for _, f := range k.presetupFamilies {
		if f == family {
			return
		}
	}
	k.presetupFamilies = append(k.presetupFamilies, family)
}

// recordCopiedPlugins is context::copy_plugins' registration order: the
// source's m_theory_set, in order. Families are matched by name, since
// translate may target another manager.
func (k *SMTKernel) recordCopiedPlugins(source *SMTKernel) error {
	// Before its setup, the source's m_theory_set holds only the plugins it
	// received through register_plugin (its own copies, ensure_pb).
	families := source.presetupFamilies
	if source.preprocessingSetupDone {
		families = families[:0:0]
		for _, theory := range source.internalizer.theoryOrder {
			families = append(families, theory.FamilyID())
		}
	}
	for _, sourceFamily := range families {
		name, err := source.manager.FamilyName(sourceFamily)
		if err != nil {
			return err
		}
		text, err := source.manager.Symbols.String(name)
		if err != nil {
			return err
		}
		local, err := k.manager.Symbols.Intern(text)
		if err != nil {
			return err
		}
		family, err := k.manager.FamilyIDForSymbol(local)
		if err != nil || family == ast.NullFamilyID {
			continue
		}
		if k.internalizer.theories[family] != nil {
			k.recordPresetupPlugin(family)
		}
	}
	return nil
}

// setupProfilePlugins is the plugin list the selected setup registers:
// CFG_BASIC's setup_unknown(), CFG_AUTO's setup_unknown(st) profile, or
// CFG_LOGIC's setup_default fallthrough to setup_unknown(). known is false
// for an explicit logic's own profile, which the constructor installed:
// only plugins registered before setup are moved ahead of it.
func (k *SMTKernel) setupProfilePlugins(useStaticFeatures bool) (plugins []int, known bool, result error) {
	switch {
	case !k.smtState.AutoConfig:
		return smtSetupAllPlugins(), true, nil
	case useStaticFeatures && smtAutoConfigUsesUnknownSetup(k.setupLogic):
		features, err := k.setupStaticFeatures(true)
		if err != nil {
			return nil, false, err
		}
		defer func() { result = errors.Join(result, features.Close()) }()
		return smtUnknownStaticPlugins(features), true, nil
	case !useStaticFeatures && smtLogicUsesUnknownSetup(k.setupLogic):
		return smtSetupAllPlugins(), true, nil
	}
	return nil, false, nil
}

// installMissingSetupPlugin registers a fresh plugin for a profile family
// the constructor did not install (setup_arrays, setup_bv, setup_datatypes,
// setup_dl, setup_card, setup_fpa). Arithmetic and sequences are installed
// by setupBasicArithmetic and setupSequenceBackend.
func (k *SMTKernel) installMissingSetupPlugin(plugin int) error {
	m := k.manager
	family, err := k.smtSetupPluginFamily(plugin)
	if err != nil {
		return err
	}
	var theory SMTTheoryInternalizer
	switch plugin {
	case smtSetupArith:
		return k.setupBasicArithmetic()
	case smtSetupArray:
		switch k.smtParams.array.state.Mode {
		case 0: // AR_NO_ARRAY
			theory, err = NewSMTDummyTheory(family, "no array")
		case 1: // AR_SIMPLE
			theory, err = NewSMTSimpleArrayTheory(family)
		case 3: // AR_FULL
			theory, err = NewSMTArrayTheory(family)
		default:
			return ast.ErrASTDomain
		}
	case smtSetupBV:
		if k.smtParams.bv.state.Mode == 0 { // BS_NO_BV
			theory, err = NewSMTDummyTheory(family, "no bit-vector")
		} else {
			theory, err = NewSMTBitVectorTheory(family)
		}
	case smtSetupDatatype:
		theory, err = NewSMTDatatypeTheory(family)
	case smtSetupDatalog:
		theory, err = NewSMTDatalogTheory(family)
	case smtSetupSeq:
		return nil
	case smtSetupPB:
		theory, err = NewSMTPseudoBooleanTheory(family)
	case smtSetupFPA:
		// setup_fpa calls setup_bv first.
		bv, err := m.RegisterBitVectorPlugin()
		if err != nil {
			return err
		}
		if k.internalizer.theories[bv] == nil {
			if err = k.installMissingSetupPlugin(smtSetupBV); err != nil {
				return err
			}
		}
		if _, err = m.RegisterArithmeticPlugin(); err != nil {
			return err
		}
		theory, err = NewSMTFloatingPointTheory(family)
	default:
		return ast.ErrASTDomain
	}
	if err != nil {
		return err
	}
	return k.RegisterTheory(theory)
}

// setupPluginSet makes the context's plugin set the one the source
// registers: plugins registered before setup (copy_plugins' source order,
// ensure_pb) first, then the setup profile's remaining plugins in its
// registration order (register_plugin skips families already present).
// Go eagerly installs constructor placeholders, so placeholders outside
// both lists are dropped (they would otherwise take final_check rotation
// slots and run their own final checks).
func (k *SMTKernel) setupPluginSet(useStaticFeatures bool) error {
	if k.smtParams == nil {
		return nil
	}
	plugins, known, err := k.setupProfilePlugins(useStaticFeatures)
	if err != nil {
		return err
	}
	if !known && len(k.presetupFamilies) == 0 {
		return nil
	}
	s := k.internalizer
	if len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	profile := make([]ast.FamilyID, 0, len(plugins))
	for _, plugin := range plugins {
		family, err := k.smtSetupPluginFamily(plugin)
		if err != nil {
			return err
		}
		profile = append(profile, family)
		if s.theories[family] == nil {
			if err = k.installMissingSetupPlugin(plugin); err != nil {
				return err
			}
		}
	}
	setupFamilies := make(map[ast.FamilyID]bool, smtSetupFPA+1)
	for plugin := smtSetupArith; plugin <= smtSetupFPA; plugin++ {
		family, err := k.smtSetupPluginFamily(plugin)
		if err != nil {
			return err
		}
		setupFamilies[family] = true
	}
	order := make([]SMTTheoryInternalizer, 0, len(s.theoryOrder))
	keep := make(map[SMTTheoryInternalizer]bool, len(s.theoryOrder))
	add := func(family ast.FamilyID) {
		if theory := s.theories[family]; theory != nil && !keep[theory] {
			keep[theory] = true
			order = append(order, theory)
		}
	}
	for _, family := range k.presetupFamilies {
		add(family)
	}
	for _, family := range profile {
		add(family)
	}
	var dropped []SMTTheoryInternalizer
	for _, theory := range s.theoryOrder {
		if keep[theory] {
			continue
		}
		if !known || !setupFamilies[theory.FamilyID()] {
			// Plugins registered outside setup keep their relative order,
			// as does an explicit logic's constructor profile.
			keep[theory] = true
			order = append(order, theory)
		} else {
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
