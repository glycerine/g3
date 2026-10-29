package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp query setup and PDR dispatch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (e *PropositionalEngine) closePDRForReplacement() error {
	if e.pdr == nil {
		return nil
	}
	archived := statistics.New()
	result := e.pdr.CollectStatistics(archived)
	e.pdrStatistics.Copy(*archived)
	result = errors.Join(result, e.pdr.Close())
	e.pdr = nil
	e.pdrRevision = 0
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
	result := closeHornModelConversions(e.pdrConversions)
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
	return result
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
	if e.params == nil || e.globals == nil {
		return lbool.Undef, fmt.Errorf("muz/fp: PDR requires parameter and global services")
	}
	if e.spacer != nil {
		if err := e.spacer.Close(); err != nil {
			return lbool.Undef, err
		}
		e.spacer = nil
		e.spacerRevision = 0
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
	if err = e.normalizeSimpleJoins(); err != nil {
		return lbool.Undef, err
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	if e.pdr != nil && e.pdrRevision == e.revision && e.pdrQuery == declaration {
		return e.pdr.Solve(e.pdrPredicate(declaration))
	}
	if e.pdr != nil {
		if err = e.closePDRForReplacement(); err != nil {
			return lbool.Undef, err
		}
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	maximumContexts, err := parameters.UInt("pdr.max_num_contexts", 500)
	if err != nil {
		return lbool.Undef, err
	}
	cacheValue, err := parameters.UInt("pdr.cache_mode", uint32(pdr.ReachableNoCache))
	if err != nil {
		return lbool.Undef, err
	}
	cacheMode := pdr.ReachableCacheMode(cacheValue)
	if cacheMode > pdr.ReachableConstraintCache {
		return lbool.Undef, fmt.Errorf("muz/fp: invalid pdr.cache_mode %d", cacheValue)
	}
	context, err := pdr.NewContext(e.manager, parameters.local, maximumContexts, cacheMode, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return lbool.Undef, err
	}
	useFarkas, err := parameters.Bool("pdr.farkas", true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseFarkas(useFarkas); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useInductive, err := parameters.Bool("pdr.use_inductive_generalizer", true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseInductiveGeneralizer(useInductive); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useMultiCore, err := parameters.Bool("pdr.use_multicore_generalizer", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseMultiCoreGeneralizer(useMultiCore); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useArithmeticInductive, err := parameters.Bool("pdr.use_arith_inductive_generalizer", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseArithmeticInductiveGeneralizer(useArithmeticInductive); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useInductiveReachability, err := parameters.Bool("pdr.inductive_reachability_check", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseInductiveReachabilityGeneralizer(useInductiveReachability); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useConvexClosure, err := parameters.Bool("pdr.use_convex_closure_generalizer", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseConvexClosureGeneralizer(useConvexClosure); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useConvexInterior, err := parameters.Bool("pdr.use_convex_interior_generalizer", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseConvexInteriorGeneralizer(useConvexInterior); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useUTVPI, err := parameters.Bool("pdr.utvpi", true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseUTVPI(useUTVPI); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	useModelGeneralizer, err := parameters.Bool("pdr.use_model_generalizer", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseModelGeneralizer(useModelGeneralizer); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	breadthFirst, err := parameters.Bool("pdr.bfs_model_search", true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetBreadthFirstModelSearch(breadthFirst); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	flexible, err := parameters.Bool("pdr.flexible_trace", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetFlexibleTrace(flexible); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	inlineEager, err := parameters.Bool("xform.inline_eager", true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	inlineLinear, err := parameters.Bool("xform.inline_linear", true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetFullTraceAnswer(!inlineEager && !inlineLinear); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	generateProofTrace, err := parameters.Bool("generate_proof_trace", false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetGenerateProofTrace(generateProofTrace || e.forceProofTrace); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	preprocessed, err := e.preprocessHornRules(declaration, true, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessed.Close()) }()
	prepared := preprocessed.rules
	e.installPDRPreprocessing(preprocessed)
	installed := false
	defer func() {
		if !installed {
			resultErr = errors.Join(resultErr, e.clearPDRPreprocessing())
		}
	}()
	e.pdrDerived = make(map[ast.ASTHandle]ast.ASTHandle, len(e.predicateOrder)+len(e.auxiliary)+1)
	for _, original := range e.predicateOrder {
		derived, deriveErr := prepared.DerivedPredicate(original)
		if deriveErr != nil {
			return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: resolve preprocessed PDR predicate: %w", deriveErr), context.Close())
		}
		e.pdrDerived[original] = derived
	}
	queryDeclaration, err := prepared.DerivedPredicate(declaration)
	if err != nil {
		return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: resolve preprocessed PDR query: %w", err), context.Close())
	}
	auxiliary := make([]ast.ASTHandle, 0, len(e.auxiliary))
	for declaration := range e.auxiliary {
		auxiliary = append(auxiliary, e.pdrPredicate(declaration))
	}
	if err = context.SetAuxiliaryPredicates(auxiliary); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	background, err := e.manager.RefGet(e.background)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetBackground(background); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.Initialize(prepared); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	e.pdr = context
	e.pdrRevision = e.revision
	e.pdrQuery = declaration
	installed = true
	return context.Solve(queryDeclaration)
}

func (e *PropositionalEngine) pdrContext() (*pdr.Context, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend != "pdr" || e.pdr == nil || e.pdrRevision != e.revision {
		return nil, ErrPropositionalEngine
	}
	return e.pdr, nil
}

// NumLevels returns the finite PDR frame count for a relation after a query.
func (e *PropositionalEngine) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := e.open(); err != nil {
		return 0, err
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		return e.spacer.NumLevels(e.spacerPredicate(declaration))
	}
	context, err := e.pdrContext()
	if err != nil {
		return 0, err
	}
	return context.NumLevels(e.pdrPredicate(declaration))
}

// CoverDelta returns an owning reference to a PDR or Spacer cover frame after
// a query.
func (e *PropositionalEngine) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		derived := e.spacerPredicate(declaration)
		_, found, err := e.spacer.Transformer(derived)
		if err != nil {
			return ast.ASTRef{}, err
		}
		cover, err := e.spacer.CoverDelta(level, derived)
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
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
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
		return e.spacer.AddCover(level, e.spacerPredicate(declaration), property)
	}
	context, err := e.pdrContext()
	if err != nil {
		return err
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
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.backend != "spacer" || e.spacer == nil || e.spacerRevision != e.revision {
		return ast.ASTRef{}, ErrPropositionalEngine
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
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.backend != "spacer" || e.spacer == nil || e.spacerRevision != e.revision {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	status, err := e.spacer.LastResult()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if status != lbool.True {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	answer, err := e.spacer.Answer(e.spacerPredicate(declaration))
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.expandSpacerFormula(declaration, answer)
}

// RulesAlongTrace returns owning rule formulas and their names in preorder for
// the latest successful Spacer query. The caller releases every ASTRef.
func (e *PropositionalEngine) RulesAlongTrace() (formulas []ast.ASTRef, names []string, resultErr error) {
	if err := e.open(); err != nil {
		return nil, nil, err
	}
	if e.backend != "spacer" || e.spacer == nil || e.spacerRevision != e.revision {
		return nil, nil, ErrPropositionalEngine
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
