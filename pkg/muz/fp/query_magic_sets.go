package fp

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// Source: src/muz/rel/rel_context.cpp:329-358.
// applyQueryMagicSets matches rel_context::query's query-local magic-set pass.
// QueryExpression supplies the synthetic one-rule query predicate used as the
// transformation root; direct QueryPredicate calls do not create that wrapper.
func (e *PropositionalEngine) applyQueryMagicSets(goal ast.ASTHandle) (resultErr error) {
	if goal.IsNull() || e.backend != "datalog" || e.params == nil || e.globals == nil {
		return nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	enabled, err := view.Bool("datalog.magic_sets_for_queries", false)
	err = errors.Join(err, view.Close())
	if err != nil || !enabled {
		return err
	}

	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if !finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("muz/fp: negation is not stratified")
		}
	}

	plugin := transforms.NewMagicSetsPluginWithRelations(e.manager, e.rules, goal, true, 0, e.relations)
	transformer := base.NewRuleTransformer()
	if err = transformer.Register(plugin); err != nil {
		return errors.Join(err, transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(e.set)
	if err != nil || !modified {
		return err
	}
	for _, declaration := range plugin.FreshPredicates() {
		state, stateErr := e.predicateState(declaration)
		if stateErr != nil {
			return stateErr
		}
		if e.predicates[declaration] == nil {
			e.predicateOrder = append(e.predicateOrder, declaration)
		}
		e.predicates[declaration] = state
		e.auxiliary[declaration] = true
	}
	e.revision++
	return nil
}
