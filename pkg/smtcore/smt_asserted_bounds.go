package smtcore

// Source: src/smt/asserted_formulas.h cheap_quant_fourier_motzkin;
// src/smt/asserted_formulas.cpp simplify_fmls, reduce.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// smtSetup owns the read-only formula statistics for one first push or check.
// Setup may install theories and adjust options. Its asserted formulas stay
// fixed until all policies have finished, including on a failed attempt.
type smtSetup struct {
	*SMTKernel
	features            *ast.StaticFeatures
	preserveAllocations bool
}

func (k *smtSetup) setupStaticFeatures(enabled bool) (*ast.StaticFeatures, error) {
	if !enabled {
		return nil, nil
	}
	if k.preserveAllocations {
		return k.collectSetupFeatures()
	}
	if k.features == nil {
		features, err := k.collectSetupFeatures()
		if err != nil {
			return nil, err
		}
		k.features = features
	}
	return k.features, nil
}

// A policy borrows the shared summary. Diagnostic and allocation-failure
// runs own a separate summary at each original collection point.
func (k *smtSetup) releaseFeatures(features *ast.StaticFeatures) error {
	if features == nil || features == k.features {
		return nil
	}
	return features.Close()
}

// Source setup_AUFLIA / setup_AUFLIRA enable this hidden preprocessor field.
// Ordinary checks and push select CFG_LOGIC. A fresh tactic's SetupAndCheck
// selects static formula features for automatic setup_unknown.
func (k *SMTKernel) setupAssertedBounds(useStaticFeatures bool) error {
	if k.preprocessingSetupDone || k.smtParams == nil {
		return nil
	}
	setup := smtSetup{
		SMTKernel: k,
		preserveAllocations: ast.ASTSourceDebug || diagnostic.VerboseProgress || k.manager.TraceStream != nil ||
			k.manager.Budget().InjectionArmed() || k.manager.NumeralService().RuntimeOwner().AllocationLimited() ||
			k.manager.ResourceLimit().Limit != 0 || k.manager.ResourceLimit().CancelFlag(),
	}
	err := setup.configure(useStaticFeatures)
	if setup.features != nil {
		err = errors.Join(err, setup.features.Close())
	}
	if err == nil {
		k.preprocessingSetupDone = true
	}
	return err
}

func (k *smtSetup) configure(useStaticFeatures bool) error {
	// setup::operator()'s (smt.configuring)/(smt.collecting-features) lines
	// (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress {
		if err := k.setupVerbose(useStaticFeatures); err != nil {
			return err
		}
	}
	if !k.smtState.AutoConfig {
		if err := k.setupBasicArithmetic(); err != nil {
			return err
		}
		if err := k.setupArrayBackend(); err != nil {
			return err
		}
		if err := k.setupSequenceBackend(useStaticFeatures); err != nil {
			return err
		}
		if err := k.setupPluginSet(useStaticFeatures); err != nil {
			return err
		}
		k.setupAssertedPolicies()
		return nil
	}
	if !useStaticFeatures && smtLogicUsesUnknownSetup(k.setupLogic) {
		if err := k.setupBasicArithmetic(); err != nil {
			return err
		}
	}
	// setup_auto_config routes QF_FP, QF_FPBV and QF_BVFP through
	// setup_unknown(st), which reaches their setups only for a pure profile.
	if !useStaticFeatures || !smtAutoConfigUsesUnknownSetup(k.setupLogic) {
		if err := k.setupBitVectorLogicPolicy(); err != nil {
			return err
		}
	}
	if err := k.setupIntegerDifferencePolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupRealDifferencePolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupUFIntegerDifferencePolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupIntegerLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupRealLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupSimpleLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupArrayLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupQuantifiedLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupUnknownStaticPolicy(useStaticFeatures); err != nil {
		return err
	}
	enabled := false
	switch k.setupLogic {
	case "AUFLIA", "AUFLIRA", "AUFNIRA", "AUFLIA+", "AUFLIA-", "AUFLIRA+", "AUFLIRA-", "AUFNIRA+", "AUFNIRA-", "UFNIA":
		enabled = true
	case "UFLRA":
		// setup_auto_config routes this logic through setup_unknown(st).
		enabled = !useStaticFeatures
	}
	if !enabled && useStaticFeatures && smtAutoConfigUsesUnknownSetup(k.setupLogic) {
		for _, row := range k.assertions {
			has, err := k.manager.HasQuantifiers(row.Formula)
			if err != nil {
				return err
			}
			if has {
				enabled = true
				break
			}
		}
	}
	if enabled {
		k.preprocessState.EliminateBounds = true
		k.smtParams.preprocess.state.EliminateBounds = true
	}
	if err := k.setupArrayBackend(); err != nil {
		return err
	}
	if err := k.setupSequenceBackend(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupPluginSet(useStaticFeatures); err != nil {
		return err
	}
	k.setupAssertedPolicies()
	return nil
}

func (k *SMTKernel) eliminatePendingBounds() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.EliminateBounds {
		return nil
	}
	if !k.assertedHasQuantifiers {
		return nil
	}
	// m_cheap_quant_fourier_motzkin owns its elim_bounds_rw for the asserted
	// formulas' lifetime; its rewriter cache persists across reduce() calls.
	if k.assertedElimBounds == nil {
		proc, err := rewriter.NewElimBounds(k.manager)
		if err != nil {
			if proc != nil {
				err = errors.Join(err, proc.Close())
			}
			return err
		}
		k.assertedElimBounds = proc
	}
	proc := k.assertedElimBounds
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	for _, row := range k.assertions[k.assertedHead:] {
		if err = proc.RewriteWithProof(row.Formula, out, proof); err != nil {
			return err
		}
		if err = k.appendSimplifiedAssertion(batch, row, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, true)
}
