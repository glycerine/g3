package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp query setup and PDR dispatch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (e *PropositionalEngine) snapshotPDRStatistics() (pdr.StatisticsSnapshot, error) {
	var result pdr.StatisticsSnapshot
	result.Merge(e.pdrStatistics)
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "pdr" || pending.clone.backend == "qpdr" {
			if err := pending.captureStatistics(); err != nil {
				return result, err
			}
			result.Merge(pending.pdr)
		}
	}
	if e.pdr != nil {
		current, err := e.pdr.SnapshotStatistics()
		if err != nil {
			return result, err
		}
		result.Merge(current)
	}
	if e.queryClone != nil && (e.queryClone.backend == "pdr" || e.queryClone.backend == "qpdr") {
		current, err := e.queryClone.snapshotPDRStatistics()
		if err != nil {
			return result, err
		}
		result.Merge(current)
	}
	return result, nil
}

func (e *PropositionalEngine) collectPDRStatistics(destination *statistics.Statistics, previous pdr.StatisticsSnapshot) error {
	previous.Merge(e.pdrStatistics)
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "pdr" || pending.clone.backend == "qpdr" {
			if err := pending.captureStatistics(); err != nil {
				return err
			}
			previous.Merge(pending.pdr)
		}
	}
	if e.queryClone != nil && (e.queryClone.backend == "pdr" || e.queryClone.backend == "qpdr") {
		if e.pdr != nil {
			current, err := e.pdr.SnapshotStatistics()
			if err != nil {
				return err
			}
			previous.Merge(current)
		}
		return e.queryClone.collectPDRStatistics(destination, previous)
	}
	if e.pdr != nil {
		return e.pdr.CollectStatisticsWithSnapshot(destination, previous)
	}
	return previous.Collect(destination)
}

func (e *PropositionalEngine) closePDRForReplacement() error {
	var result error
	if e.pdr != nil {
		archived, err := e.pdr.SnapshotStatistics()
		result = err
		if err == nil {
			e.pdrStatistics.Merge(archived)
		}
		result = errors.Join(result, e.pdr.Close())
		e.pdr = nil
	}
	if e.pdrRules != nil {
		result = errors.Join(result, e.pdrRules.Close())
		e.pdrRules = nil
	}
	if e.pdrSourceRules != nil {
		result = errors.Join(result, e.pdrSourceRules.Close())
		e.pdrSourceRules = nil
	}
	e.pdrRevision = 0
	e.pdrRuleRevision = 0
	e.pdrQuery = ast.ASTHandle{}
	result = errors.Join(result, e.clearPDRPreprocessing())
	return result
}

func (e *PropositionalEngine) pdrPredicate(declaration ast.ASTHandle) ast.ASTHandle {
	if derived, found := e.pdrDerived[declaration]; found {
		return derived
	}
	return hornDerivedPredicate(declaration, e.pdrQuantified, e.pdrScaled, e.pdrMagic, e.pdrBitBlasted, e.pdrInstantiated, e.pdrPredicates)
}

func (e *PropositionalEngine) clearPDRPreprocessing() error {
	if err := closeHornModelConversions(e.pdrConversions); err != nil {
		return err
	}
	e.pdrConversions = nil
	if e.pdrProof != (ast.ASTRef{}) {
		if err := e.manager.DelRef(&e.pdrProof); err != nil {
			return err
		}
	}
	e.pdrSliceProof = false
	e.pdrDerived = nil
	e.pdrPredicates = nil
	e.pdrSlices = nil
	e.pdrQuantified = nil
	e.pdrInstantiated = nil
	e.pdrBitBlasted = nil
	e.pdrScaled = nil
	e.pdrMagic = nil
	e.pdrKarr = nil
	e.pdrConversions = nil
	return nil
}

func (e *PropositionalEngine) installPDRPreprocessing(result *hornPreprocessResult) {
	e.pdrPredicates = result.slicedPredicates
	e.pdrSlices = result.slicedColumns
	e.pdrQuantified = result.quantifiedPredicates
	e.pdrInstantiated = result.instantiatedPredicates
	e.pdrBitBlasted = result.bitBlastedPredicates
	e.pdrScaled = result.scaledPredicates
	e.pdrMagic = result.magicPredicates
	e.pdrKarr = result.karrInvariants
	e.pdrConversions = result.takeModelConversions()
}

func (e *PropositionalEngine) queryPDR(declaration ast.ASTHandle) (_ lbool.LBool, resultErr error) {
	status, err := e.queryPDRWithSliceProof(declaration, true)
	if onlyErrorKind(err, errRetryUnslicedProof) {
		if closeErr := e.closePDRForReplacement(); closeErr != nil {
			return lbool.Undef, closeErr
		}
		return e.queryPDRWithSliceProof(declaration, false)
	}
	return status, err
}

func (e *PropositionalEngine) queryPDRWithSliceProof(declaration ast.ASTHandle, allowSliceProof bool) (_ lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, fmt.Errorf("muz/fp: PDR requires parameter and global services")
	}
	if e.spacer != nil {
		if err := e.closeSpacerForReplacement(); err != nil {
			return lbool.Undef, err
		}
		e.spacerPredicates = nil
		e.spacerSlices = nil
		e.spacerQuantified = nil
		e.spacerInstantiated = nil
		e.spacerBitBlasted = nil
		e.spacerScaled = nil
		e.spacerMagic = nil
		e.spacerKarr = nil
	}
	if err := closeHornModelConversions(e.spacerConversions); err != nil {
		return lbool.Undef, errors.Join(resultErr, err)
	}
	e.spacerConversions = nil
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, x := e.set.Finalize()
		if x != nil {
			return lbool.Undef, x
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	// Match pdr_dl_interface's default Horn transformations. Relational
	// filter/simple-join normalization belongs to the Datalog query path.
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	// Source check_reset scans original rules before default transformations.
	sourceReusable, err := e.canReusePDRSourceRules(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	if e.pdr != nil && e.pdrRevision == e.revision && e.pdrQuery == declaration && !e.pdrSliceProof {
		if err = writePDRQueryRules(e.pdrRules); err != nil {
			return lbool.Undef, err
		}
		status, err := e.pdr.Solve(e.pdrPredicate(declaration))
		if err == nil && status == lbool.False {
			err = e.writePDREmptyProgramModel(declaration)
		}
		return status, err
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	context, err := e.newConfiguredPDRContext(parameters)
	if err != nil {
		return lbool.Undef, err
	}
	closeContext := func() error {
		if context == e.pdr {
			if e.pdrRules == nil {
				initialized, err := context.Initialized()
				if err == nil && !initialized {
					return nil
				}
				if err != nil {
					return errors.Join(err, e.closePDRForReplacement())
				}
			}
			return e.closePDRForReplacement()
		}
		return context.Close()
	}
	preprocessed, err := e.preprocessHornRulesWithSliceProof(declaration, true, true, allowSliceProof)
	if err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessed.Close()) }()
	prepared := preprocessed.rules
	if err = writePDRQueryRules(prepared); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	snapshot, err := snapshotHornRules(prepared)
	if err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	sourceSnapshot, err := snapshotHornRules(e.set)
	if err != nil {
		return lbool.Undef, errors.Join(err, snapshot.Close(), closeContext())
	}
	keepSnapshot := false
	defer func() {
		if !keepSnapshot {
			resultErr = errors.Join(resultErr, snapshot.Close(), sourceSnapshot.Close())
		}
	}()
	installed := false
	defer func() {
		if !installed {
			resultErr = errors.Join(resultErr, closeHornModelConversions(preprocessed.modelConversions))
			preprocessed.modelConversions = nil
		}
	}()
	derivedPredicates := make(map[ast.ASTHandle]ast.ASTHandle, len(e.predicateOrder)+len(e.auxiliary)+1)
	for _, original := range e.predicateOrder {
		derived, deriveErr := prepared.DerivedPredicate(original)
		if deriveErr != nil {
			return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: resolve preprocessed PDR predicate: %w", deriveErr), context.Close())
		}
		derivedPredicates[original] = derived
	}
	queryDeclaration, err := prepared.DerivedPredicate(declaration)
	if err != nil {
		return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: resolve preprocessed PDR query: %w", err), context.Close())
	}
	auxiliary := make([]ast.ASTHandle, 0, len(e.auxiliary))
	for declaration := range e.auxiliary {
		derived := hornDerivedPredicate(declaration, preprocessed.quantifiedPredicates, preprocessed.scaledPredicates, preprocessed.magicPredicates, preprocessed.bitBlastedPredicates, preprocessed.instantiatedPredicates, preprocessed.slicedPredicates)
		auxiliary = append(auxiliary, derived)
	}
	reuse := sourceReusable
	if reuse {
		reuse, err = e.pdr.CanUpdateRules(prepared)
	}
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	oldSnapshot := e.pdrRules
	oldSourceSnapshot := e.pdrSourceRules
	reset := !reuse && e.pdr != nil && e.pdrRules != nil && e.pdrRuleRevision == e.revision
	if reuse {
		if err = context.Close(); err != nil {
			return lbool.Undef, err
		}
		context = e.pdr
		if err = context.UpdateRules(prepared); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
		if err = e.clearPDRPreprocessing(); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
	} else {
		if e.pdr != nil && e.pdrRules == nil {
			// Source check_reset preserves covers supplied before any old rules.
			if err = context.Close(); err != nil {
				return lbool.Undef, err
			}
			context = e.pdr
		} else if reset {
			if err = context.Close(); err != nil {
				return lbool.Undef, err
			}
			context = e.pdr
			if err = context.Reset(); err != nil {
				return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
			}
			if err = e.clearPDRPreprocessing(); err != nil {
				return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
			}
		} else if err = e.closePDRForReplacement(); err != nil {
			return lbool.Undef, errors.Join(err, closeContext())
		}
		if err = context.SetAuxiliaryPredicates(auxiliary); err != nil {
			return lbool.Undef, errors.Join(err, closeContext())
		}
		background, backgroundErr := e.manager.RefGet(e.background)
		if backgroundErr != nil {
			return lbool.Undef, errors.Join(backgroundErr, closeContext())
		}
		if err = context.SetBackground(background); err != nil {
			return lbool.Undef, errors.Join(err, closeContext())
		}
		if err = context.Initialize(prepared); err != nil {
			return lbool.Undef, errors.Join(err, closeContext())
		}
	}
	e.installPDRPreprocessing(preprocessed)
	e.pdrSliceProof = preprocessed.sliceProof != nil
	e.pdrDerived = derivedPredicates
	e.pdr = context
	e.pdrRules = snapshot
	e.pdrSourceRules = sourceSnapshot
	e.pdrRevision = e.revision
	e.pdrRuleRevision = e.revision
	e.pdrQuery = declaration
	keepSnapshot = true
	installed = true
	if (reuse || reset) && oldSnapshot != nil {
		if err = oldSnapshot.Close(); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
	}
	if (reuse || reset) && oldSourceSnapshot != nil {
		if err = oldSourceSnapshot.Close(); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
	}
	status, err := context.Solve(queryDeclaration)
	if err == nil && status == lbool.False {
		err = e.writePDREmptyProgramModel(declaration)
	}
	if err != nil || status != lbool.True || preprocessed.sliceProof == nil {
		return status, err
	}
	proof, err := context.Proof(queryDeclaration)
	if err != nil {
		if onlyErrorKind(err, ast.ErrASTDomain) {
			return lbool.Undef, fmt.Errorf("%w: context proof: %v", errRetryUnslicedProof, err)
		}
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&proof)) }()
	original, err := e.manager.RefGet(proof)
	if err != nil {
		return lbool.Undef, err
	}
	converted, err := preprocessed.sliceProof.Convert(original)
	if err != nil {
		if onlyErrorKind(err, transforms.ErrSliceProofUnsupported) || onlyErrorKind(err, ast.ErrASTDomain) {
			return lbool.Undef, fmt.Errorf("%w: %v", errRetryUnslicedProof, err)
		}
		return lbool.Undef, err
	}
	convertedHandle, err := e.manager.RefGet(converted)
	if err == nil {
		var before, after ast.ASTHandle
		before, err = e.manager.GetProofFact(original)
		if err == nil {
			after, err = e.manager.GetProofFact(convertedHandle)
		}
		if err == nil && before != after {
			err = errRetryUnslicedProof
		}
	}
	if err != nil {
		return lbool.Undef, errors.Join(err, e.manager.DelRef(&converted))
	}
	e.pdrProof = converted
	return status, nil
}

func (e *PropositionalEngine) pdrContext() (_ *pdr.Context, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend != "pdr" && e.backend != "qpdr" {
		return nil, ErrPropositionalEngine
	}
	if e.pdr != nil {
		if e.pdrRules != nil && e.pdrRevision != e.revision {
			return nil, ErrPropositionalEngine
		}
		return e.pdr, nil
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	context, err := e.newConfiguredPDRContext(parameters)
	if err != nil {
		return nil, err
	}
	background, err := e.manager.RefGet(e.background)
	if err == nil {
		err = context.SetBackground(background)
	}
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	e.pdr = context
	e.pdrRevision = e.revision
	return context, nil
}

// Source level/cover access remains valid after a failed solve. Its program
// revision is independent of the successful-result snapshot used by answers.
func (e *PropositionalEngine) propertyQueryClone() (*PropositionalEngine, error) {
	if e.queryClone == nil || e.queryCloneRevision != e.revision || e.queryClone.backend != e.backend {
		return nil, ErrPropositionalEngine
	}
	return e.queryClone, nil
}

// NumLevels returns the finite frame count for a PDR or Spacer relation.
func (e *PropositionalEngine) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := e.open(); err != nil {
		return 0, err
	}
	if e.queryClone != nil && (e.backend == "pdr" || e.backend == "qpdr" || e.backend == "spacer") {
		clone, err := e.propertyQueryClone()
		if err != nil {
			return 0, err
		}
		return clone.NumLevels(declaration)
	}
	if e.backend == "spacer" {
		context, err := e.spacerContext()
		if err != nil {
			return 0, err
		}

		return context.NumLevels(e.spacerPredicate(declaration))
	}
	context, err := e.pdrContext()
	if err != nil {
		return 0, err
	}
	return context.NumLevels(e.pdrPredicate(declaration))
}

// CoverDelta returns an owning reference to a PDR or Spacer cover frame.
func (e *PropositionalEngine) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.queryClone != nil && (e.backend == "pdr" || e.backend == "qpdr" || e.backend == "spacer") {
		clone, err := e.propertyQueryClone()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return clone.CoverDelta(level, declaration)
	}
	if e.backend == "spacer" {
		context, err := e.spacerContext()
		if err != nil {
			return ast.ASTRef{}, err
		}

		derived := e.spacerPredicate(declaration)
		_, found, err := context.Transformer(derived)
		if err != nil {
			return ast.ASTRef{}, err
		}
		cover, err := context.CoverDelta(level, derived)
		if err != nil {
			return ast.ASTRef{}, err
		}
		return e.expandHornCover(declaration, derived, cover, found, e.spacerConversions)
	}
	context, err := e.pdrContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	derived := e.pdrPredicate(declaration)
	if e.pdrRules == nil {
		return context.CoverDelta(level, derived)
	}
	_, found, err := context.Transformer(derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cover, err := context.CoverDelta(level, derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.expandHornCover(declaration, derived, cover, found, e.pdrConversions)
}

// AddCover installs a bound-variable property in a retained PDR or Spacer
// frame.
func (e *PropositionalEngine) AddCover(level int, declaration, property ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.queryClone != nil && (e.backend == "pdr" || e.backend == "qpdr" || e.backend == "spacer") {
		clone, err := e.propertyQueryClone()
		if err != nil {
			return err
		}
		return clone.AddCover(level, declaration, property)
	}
	if e.backend == "spacer" {
		context, err := e.spacerContext()
		if err != nil {
			return err
		}

		if err := e.rejectSlicedProperty(declaration); err != nil {
			return err
		}
		eliminated, err := e.hornInlineEliminated(e.spacerConversions, declaration)
		if err != nil {
			return err
		}
		if eliminated {
			return fmt.Errorf("muz/fp: covers and invariants are incompatible with eliminated predicates")
		}
		return context.AddCover(level, e.spacerPredicate(declaration), property)
	}
	context, err := e.pdrContext()
	if err != nil {
		return err
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	slicing, err := parameters.Bool("xform.slice", true)
	err = errors.Join(err, parameters.Close())
	if err != nil {
		return err
	}
	if slicing {
		return fmt.Errorf("Covers are incompatible with slicing. Disable slicing before using covers")
	}
	if slicedPredicateHasRemovedColumns(e.pdrSlices, declaration) {
		return fmt.Errorf("muz/fp: covers and invariants are incompatible with sliced predicate columns")
	}
	eliminated, err := e.hornInlineEliminated(e.pdrConversions, declaration)
	if err != nil {
		return err
	}
	if eliminated {
		return fmt.Errorf("muz/fp: covers and invariants are incompatible with eliminated predicates")
	}
	return context.AddCover(level, e.pdrPredicate(declaration), property)
}

// AddInvariant installs a bound-variable property in the invariant frame.
func (e *PropositionalEngine) AddInvariant(declaration, property ast.ASTHandle) error {
	return e.AddCover(-1, declaration, property)
}

// Reachable returns the accumulated Spacer reach summary for a predicate.
func (e *PropositionalEngine) Reachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.requireResultSnapshot(EngineSpacer); err != nil {
		return ast.ASTRef{}, err
	}
	derived := e.spacerPredicate(declaration)
	_, found, err := e.spacer.Transformer(derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err := e.spacer.Reachable(derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.expandHornCover(declaration, derived, result, found, e.spacerConversions)
}

// GroundSatAnswer returns the latest successful Spacer reach-fact answer.
func (e *PropositionalEngine) GroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.requireResultSnapshot(EngineSpacer); err != nil {
		return ast.ASTRef{}, err
	}
	if declaration != e.spacerOriginalQuery {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	answer, err := e.spacer.GroundSatAnswer(e.spacerQuery)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if answer == (ast.ASTRef{}) {
		return answer, nil
	}
	return e.expandSpacerFormula(declaration, answer)
}

// RulesAlongTrace returns owning rule formulas and their names in preorder for
// the latest successful Spacer query. The caller releases every ASTRef.
func (e *PropositionalEngine) RulesAlongTrace() (formulas []ast.ASTRef, names []string, resultErr error) {
	if e.queryClone != nil && e.backend == "spacer" {
		clone, _, err := e.expressionQueryState()
		if err != nil {
			return nil, nil, err
		}
		return clone.RulesAlongTrace()
	}
	if e.backend == "bmc" || e.backend == "qbmc" {
		return e.bmcRulesAlongTrace()
	}
	if err := e.requireResultSnapshot(EngineSpacer); err != nil {
		return nil, nil, err
	}
	rules, err := e.spacer.RulesAlongTrace()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		for index := len(rules) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, rules[index].Close())
		}
		if resultErr != nil {
			for index := len(formulas) - 1; index >= 0; index-- {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&formulas[index]))
			}
			formulas = nil
			names = nil
		}
	}()
	for _, rule := range rules {
		formula, err := rule.ToFormula()
		if err != nil {
			return formulas, names, err
		}
		name, err := rule.Name()
		if err != nil {
			return formulas, names, err
		}
		formulas = append(formulas, formula)
		names = append(names, name)
	}
	return formulas, names, resultErr
}

// newConfiguredPDRContext owns construction until every source parameter is installed.
func (e *PropositionalEngine) newConfiguredPDRContext(parameters *fixedpointParamView) (*pdr.Context, error) {
	maximumContexts, err := parameters.UInt("pdr.max_num_contexts", 500)
	if err != nil {
		return nil, err
	}
	cacheValue, err := parameters.UInt("pdr.cache_mode", uint32(pdr.ReachableNoCache))
	if err != nil {
		return nil, err
	}
	cacheMode := pdr.ReachableCacheMode(cacheValue)
	if cacheMode > pdr.ReachableConstraintCache {
		return nil, fmt.Errorf("muz/fp: invalid pdr.cache_mode %d", cacheValue)
	}
	context, err := pdr.NewContext(e.manager, parameters.local, maximumContexts, cacheMode, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return nil, err
	}
	useFarkas, err := parameters.Bool("pdr.farkas", true)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseFarkas(useFarkas); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useInductive, err := parameters.Bool("pdr.use_inductive_generalizer", true)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseInductiveGeneralizer(useInductive); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useMultiCore, err := parameters.Bool("pdr.use_multicore_generalizer", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseMultiCoreGeneralizer(useMultiCore); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useArithmeticInductive, err := parameters.Bool("pdr.use_arith_inductive_generalizer", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseArithmeticInductiveGeneralizer(useArithmeticInductive); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useInductiveReachability, err := parameters.Bool("pdr.inductive_reachability_check", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseInductiveReachabilityGeneralizer(useInductiveReachability); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useConvexClosure, err := parameters.Bool("pdr.use_convex_closure_generalizer", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseConvexClosureGeneralizer(useConvexClosure); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useConvexInterior, err := parameters.Bool("pdr.use_convex_interior_generalizer", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseConvexInteriorGeneralizer(useConvexInterior); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useUTVPI, err := parameters.Bool("pdr.utvpi", true)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseUTVPI(useUTVPI); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	useModelGeneralizer, err := parameters.Bool("pdr.use_model_generalizer", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetUseModelGeneralizer(useModelGeneralizer); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	breadthFirst, err := parameters.Bool("pdr.bfs_model_search", true)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetBreadthFirstModelSearch(breadthFirst); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	flexible, err := parameters.Bool("pdr.flexible_trace", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetFlexibleTrace(flexible); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	inlineEager, err := parameters.Bool("xform.inline_eager", true)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	inlineLinear, err := parameters.Bool("xform.inline_linear", true)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetFullTraceAnswer(!inlineEager && !inlineLinear); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	generateProofTrace, err := parameters.Bool("generate_proof_trace", false)
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	if err = context.SetGenerateProofTrace(generateProofTrace || e.forceProofTrace); err != nil {
		return nil, errors.Join(err, context.Close())
	}
	return context, nil
}
