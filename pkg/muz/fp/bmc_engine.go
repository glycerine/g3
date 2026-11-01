package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp and fixedpoint engine registration.

import (
	"errors"
	"fmt"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var errRetryUnslicedProof = errors.New("muz/fp: retry proof without slicing")

// onlyErrorKind ensures cleanup errors are not hidden by a retry signal.
func onlyErrorKind(err, target error) bool {
	if err == nil {
		return false
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyErrorKind(child, target) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return onlyErrorKind(one.Unwrap(), target)
	}
	return err == target
}

// queryBMCRelation constructs the same synthetic query rule used for an
// expression query, preserving the source query predicate in proof answers.
func (e *PropositionalEngine) queryBMCRelation(declaration ast.ASTHandle) (status lbool.LBool, resultErr error) {
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	arguments := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sort, err := e.manager.GetDomain(declaration, i)
		if err != nil {
			return lbool.Undef, err
		}
		arguments[i], err = e.manager.MkVar(arity-i-1, sort)
		if err != nil {
			return lbool.Undef, err
		}
	}
	query, err := e.manager.MkApp(declaration, arguments)
	if err != nil {
		return lbool.Undef, err
	}
	ref, err := e.manager.NewRef(query)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&ref)) }()
	return e.QueryExpression(query)
}

func (e *PropositionalEngine) clearBMCTrace() (result error) {
	if e.bmcProof != (ast.ASTRef{}) {
		if err := e.manager.DelRef(&e.bmcProof); err != nil {
			return err
		}
	}
	for len(e.bmcTrace) != 0 {
		i := len(e.bmcTrace) - 1
		if err := e.bmcTrace[i].Close(); err != nil {
			return errors.Join(result, err)
		}
		e.bmcTrace = e.bmcTrace[:i]
	}
	return result
}

func (e *PropositionalEngine) queryBMCPredicate(declaration ast.ASTHandle) (status lbool.LBool, resultErr error) {
	status, resultErr = e.queryBMCPredicateWithSliceProof(declaration, true)
	if onlyErrorKind(resultErr, errRetryUnslicedProof) {
		return e.queryBMCPredicateWithSliceProof(declaration, false)
	}
	return status, resultErr
}

func (e *PropositionalEngine) queryBMCPredicateWithSliceProof(declaration ast.ASTHandle, allowSliceProof bool) (status lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	if err := e.clearBMCTrace(); err != nil {
		return lbool.Undef, err
	}
	e.bmcStatistics.Reset()
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	// BMC uses the same default Horn transformations and optional slicing as
	// Spacer before it selects the linear or nonlinear compiler.
	preprocessed, err := e.preprocessHornRulesWithSliceProof(declaration, true, true, allowSliceProof && e.backend == "qbmc")
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessed.Close()) }()
	rules := preprocessed.rules
	declaration, err = rules.DerivedPredicate(declaration)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: resolve preprocessed BMC query: %w", err)
	}
	group, err := rules.PredicateRules(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	for i := len(group) - 1; i >= 0; i-- {
		err = errors.Join(err, group[i].Close())
	}
	if err != nil {
		return lbool.Undef, err
	}
	if len(group) == 0 {
		return lbool.False, nil
	}
	linear, err := bmc.IsLinear(rules)
	if err != nil {
		return lbool.Undef, err
	}
	if preprocessed.sliceProof != nil && !linear {
		return lbool.Undef, errRetryUnslicedProof
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	depth, err := view.UInt("bmc.linear_unrolling_depth", math.MaxUint32)
	if err != nil {
		return lbool.Undef, err
	}
	proofTrace, err := view.Bool("generate_proof_trace", false)
	if err != nil {
		return lbool.Undef, err
	}
	local, err := e.params.NewValue()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.params.Del(&local)) }()
	quantified := linear && e.backend == "qbmc"
	if linear {
		name := "mbqi"
		if err = e.params.SetBoolName(&local, &name, quantified); err != nil {
			return lbool.Undef, err
		}
	}
	if quantified {
		name := "compact"
		if err = e.params.SetBoolName(&local, &name, true); err != nil {
			return lbool.Undef, err
		}
	}
	relevancy := "relevancy"
	value := uint32(2)
	if linear && !quantified {
		value = 0
	}
	if err = e.params.SetUIntName(&local, &relevancy, value); err != nil {
		return lbool.Undef, err
	}
	solver, err := smtcore.NewSMTSolver(e.manager, &local, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, solver.CollectStatistics(&e.bmcStatistics), solver.Close())
	}()
	compiler, err := bmc.NewCompiler(e.manager)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, compiler.Close()) }()
	var level uint32
	var md *model.ModelCore
	if quantified {
		status, level, md, err = compiler.SearchQuantifiedLinear(rules, declaration, solver)
	} else if linear {
		status, level, err = compiler.SearchLinear(rules, declaration, solver, depth)
	} else {
		status, level, err = compiler.SearchNonlinear(rules, declaration, solver)
	}
	if err != nil || status != lbool.True {
		return status, err
	}
	if !quantified {
		md, err = solver.AcquireModel()
		if err != nil {
			return lbool.Undef, err
		}
	}
	if md == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	if quantified {
		cooperation, e2 := cooperate.NewCooperation(e.manager.Budget())
		if e2 != nil {
			return lbool.Undef, e2
		}
		defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
		services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: cooperation}
		e.bmcTrace, err = compiler.QuantifiedLinearTrace(rules, declaration, level, md, services)
		if err == nil {
			e.bmcProof, err = compiler.QuantifiedLinearProof(rules, declaration, level, md, services)
		}
	} else if linear {
		e.bmcTrace, err = compiler.LinearTrace(rules, declaration, level, md)
		if err == nil && !proofTrace && !e.forceProofTrace {
			services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals}
			e.bmcProof, err = compiler.LinearProof(rules, declaration, level, md, services)
		}
	} else {
		cooperation, e2 := cooperate.NewCooperation(e.manager.Budget())
		if e2 != nil {
			return lbool.Undef, e2
		}
		defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
		services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: cooperation}
		e.bmcTrace, err = compiler.NonlinearTrace(rules, declaration, level, md, services)
	}
	if err != nil {
		return lbool.Undef, err
	}
	if preprocessed.sliceProof != nil && e.bmcProof != (ast.ASTRef{}) {
		original, cause := e.manager.RefGet(e.bmcProof)
		if cause != nil {
			return lbool.Undef, cause
		}
		converted, cause := preprocessed.sliceProof.Convert(original)
		if cause != nil {
			if onlyErrorKind(cause, transforms.ErrSliceProofUnsupported) {
				return lbool.Undef, errRetryUnslicedProof
			}
			return lbool.Undef, cause
		}
		newProof, cause := e.manager.RefGet(converted)
		if cause == nil {
			var before, after ast.ASTHandle
			before, cause = e.manager.GetProofFact(original)
			if cause == nil {
				after, cause = e.manager.GetProofFact(newProof)
			}
			if cause == nil && before != after {
				cause = errRetryUnslicedProof
			}
		}
		if cause != nil {
			return lbool.Undef, errors.Join(cause, e.manager.DelRef(&converted))
		}
		if cause = e.manager.DelRef(&e.bmcProof); cause != nil {
			return lbool.Undef, errors.Join(cause, e.manager.DelRef(&converted))
		}
		e.bmcProof = converted
	}
	return status, nil
}

// bmcRulesAlongTrace returns caller-owned formulas from the latest query.
func (e *PropositionalEngine) bmcRulesAlongTrace() (formulas []ast.ASTRef, names []string, resultErr error) {
	if err := e.requireBMCSnapshot(); err != nil {
		return nil, nil, err
	}
	defer func() {
		if resultErr != nil {
			for i := len(formulas) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&formulas[i]))
			}
			formulas, names = nil, nil
		}
	}()
	for _, rule := range e.bmcTrace {
		formula, err := rule.ToFormula()
		if err != nil {
			return formulas, names, err
		}
		formulas = append(formulas, formula)
		name, err := rule.Name()
		if err != nil {
			return formulas, names, err
		}
		names = append(names, name)
	}
	return formulas, names, nil
}
