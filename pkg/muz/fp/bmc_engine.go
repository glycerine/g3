package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp and fixedpoint engine registration.

import (
	"errors"
	"fmt"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

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
	preprocessed, err := e.preprocessHornRules(declaration, true, true)
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
	if linear {
		name := "mbqi"
		if err = e.params.SetBoolName(&local, &name, false); err != nil {
			return lbool.Undef, err
		}
	}
	relevancy := "relevancy"
	value := uint32(2)
	if linear {
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
	if linear {
		status, level, err = compiler.SearchLinear(rules, declaration, solver, depth)
	} else {
		status, level, err = compiler.SearchNonlinear(rules, declaration, solver)
	}
	if err != nil || status != lbool.True {
		return status, err
	}
	md, err := solver.AcquireModel()
	if err != nil {
		return lbool.Undef, err
	}
	if md == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	if linear {
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
	return status, nil
}

// bmcRulesAlongTrace returns caller-owned formulas from the latest query.
func (e *PropositionalEngine) bmcRulesAlongTrace() (formulas []ast.ASTRef, names []string, resultErr error) {
	if err := e.requireResultSnapshot(EngineBMC); err != nil {
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
