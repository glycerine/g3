package fp

// Source: src/muz/rel/rel_context.cpp, expression query explanation pipeline.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) datalogExplanationsEnabled() (bool, error) {
	if e.backend != "datalog" || e.params == nil || e.globals == nil {
		return false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, err
	}
	enabled, err := view.Bool("datalog.generate_explanations", false)
	return enabled, errors.Join(err, view.Close())
}

func (e *PropositionalEngine) queryDatalogExplanation(declarations []ast.ASTHandle) (status lbool.LBool, resultErr error) {
	if len(declarations) != 1 {
		return lbool.Undef, ErrPropositionalEngine
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	relationLevel, err := view.Bool("datalog.explanations_on_relation_level", false)
	magic := false
	if err == nil {
		magic, err = view.Bool("datalog.magic_sets_for_queries", false)
	}
	err = errors.Join(err, view.Close())
	if err != nil {
		return lbool.Undef, err
	}
	// Formula-backed check_relation remains a distinct whole-relation checker;
	// the concrete default-table checker below checks only finite/index storage.
	_, formulaRequested, err := e.datalogCheckRelationName()
	if err != nil {
		return lbool.Undef, err
	}
	var formulaScope *datalogExplanationFormulaScope
	var checkedScope *datalogExplanationCheckedScope
	if formulaRequested {
		// Native fact-level explanation products mix the proof index into joins
		// that the formula checker cannot safely own. Relation-level checking
		// wraps only the independent finite sieve.
		if !relationLevel {
			return lbool.Undef, rel.ErrCompilerUnsupported
		}
		formulaScope, _, err = e.beginDatalogExplanationFormulaScope()
		if err != nil {
			return lbool.Undef, err
		}
		defer func() { resultErr = errors.Join(resultErr, formulaScope.Close()) }()
	} else {
		checkedScope, _, err = e.beginDatalogExplanationCheckedScope()
		if err != nil {
			return lbool.Undef, err
		}
		if checkedScope != nil {
			defer func() { resultErr = errors.Join(resultErr, checkedScope.Close()) }()
		}
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	if err = e.relations.ResetSaturatedMarks(); err != nil {
		return lbool.Undef, err
	}
	if err = e.resetNegationDependentTables(); err != nil {
		return lbool.Undef, err
	}
	transformer, err := rel.NewExplanationTransformer(e.manager, e.rules)
	if err != nil {
		return lbool.Undef, err
	}
	owners := &datalogExplanationOwners{transformer: transformer}
	defer func() {
		if err := owners.Close(); err != nil {
			e.datalogExplanationCleanup = append(e.datalogExplanationCleanup, owners)
			resultErr = errors.Join(resultErr, err)
		}
	}()
	transformed, err := transformer.TransformRulesOnly(e.set)
	if err != nil {
		return lbool.Undef, err
	}
	if transformed == nil {
		value, err := e.manager.MkFalse()
		if err != nil {
			return lbool.Undef, err
		}
		e.datalogExplanationAnswer, err = e.manager.NewRef(value)
		return lbool.False, err
	}
	owners.set = transformed
	if relationLevel {
		if formulaScope != nil {
			err = formulaScope.transferFacts(transformer, e.set, transformed)
		} else {
			err = transformer.TransferRelationFacts(e.set, transformed, e.relations, e.predicateOrder)
		}
	} else {
		err = transformer.TransferFiniteFacts(e.set, transformed, e.relations, e.predicateOrder)
	}
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: transfer explanation facts: %w", err)
	}
	// Check the unpruned program: magic sets may erase the explained producer
	// that distinguishes a derived negative predicate from extensional facts.
	if err = rel.ValidateExplanationNegativeRelations(transformed); err != nil {
		return lbool.Undef, err
	}
	explained, err := transformer.ExplanationDeclaration(declarations[0])
	if err != nil {
		return lbool.Undef, err
	}
	if magic {
		valid, err := transformed.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, rel.ErrCompilerUnsupported
		}
		plugin := transforms.NewMagicSetsPluginWithRelations(e.manager, e.rules, explained, true, 0, e.relations)
		pass := base.NewRuleTransformer()
		owners.pass = pass
		if err = pass.Register(plugin); err != nil {
			return lbool.Undef, err
		}
		if _, err = pass.Apply(transformed); err != nil {
			return lbool.Undef, err
		}
	}
	var execution, termination *rel.InstructionBlock
	if relationLevel {
		execution, termination, err = rel.CompileRelationExplanationRulesWithGuard(e.relations, transformed, e.guard)
	} else {
		execution, termination, err = rel.CompileExplanationRulesWithGuard(e.relations, transformed, e.guard)
	}
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: compile explanation query: %w", err)
	}
	owners.execution, owners.termination = execution, termination
	context, err := rel.NewExecutionContextWithRelations(e.relations, nil)
	if err != nil {
		return lbool.Undef, err
	}
	owners.context = context
	e.compiled = true
	complete, err := execution.Perform(context)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: execute explanation query: %w", err)
	}
	if !complete {
		return lbool.Undef, ErrPropositionalEngine
	}
	complete, err = termination.Perform(context)
	if err != nil {
		return lbool.Undef, err
	}
	if !complete {
		return lbool.Undef, ErrPropositionalEngine
	}
	e.datalogStats.Reset()
	if err = execution.ProcessAllCosts(); err != nil {
		return lbool.Undef, err
	}
	if err = execution.CollectStatistics(&e.datalogStats); err != nil {
		return lbool.Undef, err
	}
	if err = context.CollectStatistics(&e.datalogStats); err != nil {
		return lbool.Undef, err
	}
	for declaration, state := range e.predicates {
		table, found, err := e.relations.TryGetFiniteRelation(declaration)
		if err != nil {
			return lbool.Undef, err
		}
		if found {
			state.table = table
		}
	}
	if relationLevel {
		product, err := e.relations.GetRelationExplanation(explained)
		if err != nil {
			return lbool.Undef, err
		}
		if formulaScope != nil {
			if err = formulaScope.requireProduct(declarations[0], product); err != nil {
				return lbool.Undef, err
			}
		} else if err = checkedScope.requireProduct(product); err != nil {
			return lbool.Undef, err
		}
		empty, err := product.Empty()
		if err != nil {
			return lbool.Undef, err
		}
		e.datalogExplanationAnswer, err = product.ToFormula()
		return lbool.FromBool(!empty), err
	}
	product, err := e.relations.GetExplanationRelation(explained)
	if err != nil {
		return lbool.Undef, err
	}
	if err = checkedScope.requireProduct(product); err != nil {
		return lbool.Undef, err
	}
	count, err := product.Len()
	if err != nil {
		return lbool.Undef, err
	}
	e.datalogExplanationAnswer, err = product.ToFormula()
	return lbool.FromBool(count != 0), err
}
