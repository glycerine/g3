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
		// Source check_relation wraps the table plugin selected for the finite
		// sieve. When default_table_checked is also active, preserve that inner
		// concrete pair instead of replacing it with the outer formula oracle.
		if formulaScope.nested {
			checkedScope, _, err = e.beginDatalogExplanationCheckedScope()
			if err != nil {
				return lbool.Undef, err
			}
			if checkedScope == nil {
				return lbool.Undef, fmt.Errorf("%w: explanation check relation requires an active checked table pair", rel.ErrCompilerUnsupported)
			}
			defer func() { resultErr = errors.Join(resultErr, checkedScope.Close()) }()
		}
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
	var magicPredicates []ast.ASTHandle
	if magic {
		var finiteOrigins map[ast.ASTHandle]ast.ASTHandle
		if !relationLevel {
			finiteOrigins, err = e.datalogExplanationFiniteOrigins(transformer)
			if err != nil {
				return lbool.Undef, err
			}
		}
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
		magicPredicates = append(magicPredicates, plugin.FreshPredicates()...)
		if !relationLevel {
			if err = e.installDatalogExplanationMagicFinite(plugin, finiteOrigins); err != nil {
				return lbool.Undef, err
			}
		}
		if formulaScope != nil && formulaScope.nested {
			if err = formulaScope.installFreshProducts(magicPredicates); err != nil {
				return lbool.Undef, err
			}
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
	if formulaScope != nil && formulaScope.nested {
		for _, declaration := range magicPredicates {
			product, found, err := e.relations.TryGetRelationExplanation(declaration)
			if err != nil {
				return lbool.Undef, err
			}
			if !found {
				continue
			}
			if err = formulaScope.requireConcreteProduct(product); err != nil {
				return lbool.Undef, fmt.Errorf("muz/fp: magic explanation product %v: %w", declaration, err)
			}
			if err = checkedScope.requireProduct(product); err != nil {
				return lbool.Undef, fmt.Errorf("muz/fp: magic explanation product %v: %w", declaration, err)
			}
		}
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
			if checkedScope != nil {
				if err = checkedScope.requireProduct(product); err != nil {
					return lbool.Undef, err
				}
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

// datalogExplanationFiniteOrigins records the source predicate behind each
// fact-level explanation declaration. MagicSets creates adorned and magic
// predicates from these declarations, and source relation-kind inheritance
// follows that direct-origin chain.
func (e *PropositionalEngine) datalogExplanationFiniteOrigins(transformer *rel.ExplanationTransformer) (map[ast.ASTHandle]ast.ASTHandle, error) {
	origins := make(map[ast.ASTHandle]ast.ASTHandle, len(e.predicateOrder))
	for _, original := range e.predicateOrder {
		explained, err := transformer.ExplanationDeclaration(original)
		if err != nil {
			return nil, err
		}
		origins[explained] = original
	}
	return origins, nil
}

// installDatalogExplanationMagicFinite preselects the original predicate's
// explicit finite representation for proof-free MagicSets declarations. A
// declaration that still carries a rule-proof sort remains owned by the
// explanation product compiler; zero-column Sparse families use source's Hash
// compatibility fallback.
func (e *PropositionalEngine) installDatalogExplanationMagicFinite(plugin *transforms.MagicSetsPlugin, origins map[ast.ASTHandle]ast.ASTHandle) error {
	for _, declaration := range plugin.FreshPredicates() {
		origin, found := plugin.FreshPredicateOrigin(declaration)
		if !found {
			continue
		}
		original, found := origins[origin]
		if !found {
			continue
		}
		// FreshPredicates is creation ordered, so publishing each resolved
		// link also resolves later magic predicates created from adorned ones.
		origins[declaration] = original
		state := e.predicates[original]
		if state == nil || !state.explicitFiniteRepresentation || len(state.requestedRepresentations) != 1 {
			continue
		}
		kind := state.requestedRepresentations[0]
		switch kind {
		case "tr_hashtable", "tr_sparse", "tr_lazy_sparse":
		default:
			continue
		}
		arity, err := e.manager.GetArity(declaration)
		if err != nil {
			return err
		}
		proofFree := true
		for column := uint32(0); column < arity; column++ {
			sort, err := e.manager.GetDomain(declaration, column)
			if err != nil {
				return err
			}
			proof, err := e.manager.IsDatalogRuleSort(sort)
			if err != nil {
				return err
			}
			if proof {
				proofFree = false
				break
			}
		}
		if !proofFree {
			continue
		}
		_, signature, err := e.relations.PredicateSignature(declaration)
		if err != nil {
			return err
		}
		var table rel.FiniteTable
		if kind == "tr_hashtable" || signature.Len() == 0 {
			table = rel.NewHashTable(signature)
		} else if kind == "tr_sparse" {
			table, err = rel.NewSparseTable(signature, 0)
		} else {
			if e.lazyFinite == nil {
				e.lazyFinite = rel.NewLazySparseFiniteFactory()
			}
			table, err = e.lazyFinite.New(signature)
		}
		if err != nil {
			return err
		}
		if err = e.relations.StoreFiniteRelation(declaration, table); err != nil {
			return errors.Join(err, table.Close())
		}
	}
	return nil
}
