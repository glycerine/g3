package fp

// Source: src/muz/rel/rel_context.cpp rel_context::rel_context, which
// registers bound_relation_plugin and interval_relation_plugin after the
// table plugins (the favourite relation plugin is the one named by
// datalog.default_relation), and relation_manager::get_relation, which
// creates an Int/Real predicate's relation with the first plugin that
// handles its signature. Such predicates are evaluated by the compiled rule
// pipeline (compiler -> instructions -> relation_manager::mk_*_fn).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"

	"github.com/glycerine/g3/pkg/muz/rel"
)

// ensureArithRelationPlugins registers the arithmetic relation plugins once
// per relation manager, as rel_context's constructor does.
func (e *PropositionalEngine) ensureArithRelationPlugins() error {
	if e.arithRelationPlugins {
		return nil
	}
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return err
	}
	if err := e.relations.SetDefaultRelation(e.datalogRelationFavourite); err != nil {
		return err
	}
	if err := e.registerRelContextPlugins(); err != nil {
		return err
	}
	e.arithRelationPlugins = true
	return nil
}

// arithRelationsRoute decides whether the query runs through the compiled
// pipeline with relation plugins: every predicate gets a relation from
// get_relation (a table_relation or an arithmetic plugin relation), at least
// one of them an arithmetic relation. Configurations that still need the
// product, karr, udoc, explanation, external or check relations keep their
// evaluators.
func (e *PropositionalEngine) arithRelationsRoute() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	switch e.datalogRelationFavourite {
	case "doc", "karr_relation", "check_relation":
		return false, nil
	}
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		return false, err
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return false, err
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return false, err
		}
		karr, err := view.Bool("xform.karr", false)
		if err = errors.Join(err, view.Close()); err != nil || karr {
			return false, err
		}
	}
	var requested []rel.RelationPlugin
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state == nil || state.external != nil || state.explicitFiniteRepresentation {
			return false, nil
		}
		switch {
		case len(state.requestedRepresentations) == 0:
		case len(state.requestedRepresentations) == 1 && (state.requestedRepresentations[0] == "bound_relation" || state.requestedRepresentations[0] == "interval_relation"):
		default:
			return false, nil
		}
		if state.table == nil && !rel.CanHandleIntervalSorts(e.manager, state.sorts) {
			return false, nil
		}
		// A relation another plugin's evaluation created (udoc,
		// check_relation) stays on that plugin's path.
		if relation, ok := rel.AsRelation(state.table); ok {
			switch relation.RelationPlugin().Name() {
			case "bound_relation", "interval_relation":
			default:
				return false, nil
			}
		}
	}
	if err := e.ensureArithRelationPlugins(); err != nil {
		return false, err
	}
	// rel_context::set_predicate_representation: a single requested kind
	// is the predicate's requested relation kind.
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if len(state.requestedRepresentations) == 1 {
			plugin := e.relations.GetRelationPlugin(state.requestedRepresentations[0])
			requested = append(requested, plugin)
			if err := e.relations.SetRequestedRelationKind(d, plugin); err != nil {
				return false, err
			}
			if state.table != nil {
				if _, isRelation := rel.AsRelation(state.table); !isRelation {
					// A table placeholder chosen before the kind was known.
					relation, err := e.relations.MkEmptyRelation(state.sorts, d)
					if err != nil {
						return false, err
					}
					if err = e.relations.ReplaceFiniteRelation(d, relation); err != nil {
						return false, errors.Join(err, relation.Close())
					}
					state.table = relation
				}
			}
		}
	}
	_ = requested
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state.table != nil {
			table, err := e.relations.ReplacePlaceholderWithRelation(d)
			if err != nil {
				return false, err
			}
			state.table = table
			continue
		}
		table, err := e.relations.ReserveFiniteRelation(d)
		if err != nil {
			return false, err
		}
		state.table = table
	}
	return true, nil
}

// addRoutedFact is rel_context::add_fact on the compiled path:
// reset_saturated_marks, then get_relation(pred).add_fact(fact). handled is
// false when the query does not run with the relation plugins or the
// predicate's relation is a table (the table fact path applies).
func (e *PropositionalEngine) addRoutedFact(declaration ast.ASTHandle, values []ast.ASTHandle) (bool, error) {
	routed, err := e.arithRelationsRoute()
	if err != nil || !routed {
		return false, err
	}
	state := e.predicates[declaration]
	relation, ok := rel.AsRelation(state.table)
	if !ok {
		return false, nil
	}
	adder, ok := relation.(rel.RelationAddFactAST)
	if !ok {
		return true, rel.ErrRelationNotTable
	}
	if err = e.relations.ResetSaturatedMarks(); err != nil {
		return true, err
	}
	if err = e.relations.MaterializeFiniteRelation(declaration); err != nil {
		return true, err
	}
	if err = adder.AddFactAST(values); err != nil {
		return true, err
	}
	e.invalidateQueryForMutation()
	e.advanceRuleRevision()
	e.compiled = false
	return true, nil
}

// addRoutedTableFact is rel_context::add_fact(pred, table_fact) for a
// relation that is not a table: table_fact_to_relation (mk_numeral) and
// add_fact.
func (e *PropositionalEngine) addRoutedTableFact(declaration ast.ASTHandle, values []uint32) (handled bool, resultErr error) {
	routed, err := e.arithRelationsRoute()
	if err != nil || !routed {
		return false, err
	}
	state := e.predicates[declaration]
	if _, ok := rel.AsRelation(state.table); !ok {
		return false, nil
	}
	refs := make([]ast.ASTRef, 0, len(values))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&refs[i]))
		}
	}()
	handles := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		h, err := e.manager.MkDatalogNumeral(uint64(value), state.sorts[i])
		if err != nil {
			return true, err
		}
		ref, err := e.manager.NewRef(h)
		if err != nil {
			return true, err
		}
		refs = append(refs, ref)
		handles[i] = h
	}
	return e.addRoutedFact(declaration, handles)
}
