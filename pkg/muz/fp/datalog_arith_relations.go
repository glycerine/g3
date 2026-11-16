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
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"

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
// one of them an arithmetic relation. With datalog.check_relation the
// favourite relation plugin's route (selectCompiledRelationFavourite) takes
// the program. Configurations that still need the karr_relation favourite,
// udoc, explanation or external relations keep their evaluators. With
// fixedpoint.xform.karr the mk_karr_invariants transform runs as before.
func (e *PropositionalEngine) arithRelationsRoute() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	switch e.datalogRelationFavourite {
	case "doc":
		return false, nil
	}
	// rel_context::updt_params: with datalog.check_relation the favourite
	// is check_relation over the named plugin; the compiled route of the
	// favourite relation plugin takes every predicate.
	if _, enabled, err := e.datalogCheckRelationBase(); err != nil || enabled {
		if err != nil {
			return false, err
		}
		// With datalog.generate_explanations the explanation evaluators
		// take the facts and queries.
		if requested, err := e.datalogExplanationsRequested(); err != nil || requested {
			return false, err
		}
		return e.selectCompiledRelationFavourite()
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return false, err
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	var requested []rel.RelationPlugin
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state == nil || state.external != nil {
			return false, nil
		}
		switch {
		case len(state.requestedRepresentations) == 0:
		case state.explicitFiniteRepresentation && len(state.requestedRepresentations) == 1 && isTableRelationRepresentation(state.requestedRepresentations):
			// rel_context::set_predicate_representation of one table
			// plugin: the predicate's table_relation of that plugin.
		case len(state.requestedRepresentations) == 1 && (state.requestedRepresentations[0] == "bound_relation" || state.requestedRepresentations[0] == "interval_relation" || state.requestedRepresentations[0] == "karr_relation"):
		case isArithProductRepresentation(state.requestedRepresentations):
		case state.table == nil && isTableRelationRepresentation(state.requestedRepresentations):
		case hasExternalRepresentation(state.requestedRepresentations):
			// external_relation_plugin handles every signature.
		case e.externalPluginRegistered() && isTableRelationRepresentation(state.requestedRepresentations):
			// A requested table plugin that cannot handle the signature:
			// mk_empty_relation passes over it (the favourite or a table).
		default:
			return false, nil
		}
		external := hasExternalRepresentation(state.requestedRepresentations) || e.datalogRelationFavourite == "external_relation" && e.externalPluginRegistered()
		if state.table == nil && !external && !e.relationColumnSorts(state.sorts) {
			return false, nil
		}
		// A relation another plugin's evaluation created (udoc,
		// check_relation) stays on that plugin's path.
		if relation, ok := rel.AsRelation(state.table); ok {
			switch relation.RelationPlugin().Name() {
			case "bound_relation", "interval_relation", "karr_relation", "product_relation", "external_relation":
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
		if len(state.requestedRepresentations) > 1 {
			// product_relation_plugin::get_relation_kind(sig, rel_kinds)
			kinds := make([]int, 0, len(state.requestedRepresentations))
			for _, name := range state.requestedRepresentations {
				kind, ok := e.relations.RelationPluginKind(name)
				if !ok {
					return false, fmt.Errorf("relation plugin %s does not exist", name)
				}
				kinds = append(kinds, kind)
			}
			kind := e.relations.ProductRelationKind(state.sorts, kinds)
			if err := e.relations.SetRequestedKind(d, kind); err != nil {
				return false, err
			}
			if err := e.replaceWithRequestedRelation(d, state, kind); err != nil {
				return false, err
			}
			continue
		}
		if len(state.requestedRepresentations) == 1 && !state.explicitFiniteRepresentation && !isTableRelationRepresentation(state.requestedRepresentations) {
			plugin := e.relations.GetRelationPlugin(state.requestedRepresentations[0])
			requested = append(requested, plugin)
			if err := e.relations.SetRequestedRelationKind(d, plugin); err != nil {
				return false, err
			}
			if state.table != nil {
				if _, isRelation := rel.AsRelation(state.table); !isRelation {
					// A table placeholder chosen before the kind was known.
					relation, err := e.mkEmptyRequestedRelation(d, state)
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
		if state.table != nil && state.explicitFiniteRepresentation {
			// mk_empty_relation(sig, pred): the requested table plugin that
			// handles the signature comes before the favourite.
			continue
		}
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

// relationColumnSorts reports that every column is a table column or an
// Int/Real column: get_relation's mk_empty_relation gives a predicate of
// such a signature a table_relation, an arithmetic plugin relation, or
// the empty product relation (a mixed signature).
func (e *PropositionalEngine) relationColumnSorts(sorts []ast.ASTHandle) bool {
	for _, sort := range sorts {
		if _, table := e.relations.SortDomain(sort); !table && !rel.CanHandleIntervalSorts(e.manager, []ast.ASTHandle{sort}) {
			return false
		}
	}
	return true
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

// replaceWithRequestedRelation replaces a predicate's table or relation
// created before its requested kind was known by mk_empty_relation(sig,
// pred) of that kind.
func (e *PropositionalEngine) replaceWithRequestedRelation(d ast.ASTHandle, state *predicateState, kind int) error {
	if state.table == nil {
		return nil
	}
	if _, ok := rel.AsRelation(state.table); ok && e.relations.RelationKindOf(state.table) == kind {
		// Already the requested kind's relation.
		return nil
	}
	if n, err := state.table.Len(); err != nil || n != 0 {
		return err
	}
	relation, err := e.mkEmptyRequestedRelation(d, state)
	if err != nil {
		return err
	}
	if err = e.relations.ReplaceFiniteRelation(d, relation); err != nil {
		return errors.Join(err, relation.Close())
	}
	state.table = relation
	return nil
}

// mkEmptyRequestedRelation is mk_empty_relation(sig, pred) for a predicate
// whose table was made before its requested kind was known; a placeholder's
// relation is created by get_relation (its external relation's OP_RA_EMPTY
// runs then).
func (e *PropositionalEngine) mkEmptyRequestedRelation(d ast.ASTHandle, state *predicateState) (rel.FiniteTable, error) {
	if e.relations.IsPlaceholderRelation(d) {
		return e.relations.MkEmptyPlaceholderRelation(state.sorts, d)
	}
	return e.relations.MkEmptyRelation(state.sorts, d)
}

// hasExternalRepresentation: set_predicate_representation names the
// external relation plugin (alone or in a product_relation kind).
func hasExternalRepresentation(kinds []string) bool {
	for _, k := range kinds {
		if k == "external_relation" {
			return true
		}
	}
	return false
}

// isArithProductRepresentation reports a set_predicate_representation of
// several kinds among the table_relation, bound_relation and
// interval_relation plugins, at least one of them arithmetic: a
// product_relation kind.
func isArithProductRepresentation(kinds []string) bool {
	if len(kinds) < 2 {
		return false
	}
	arith := false
	for _, k := range kinds {
		switch k {
		case "bound_relation", "interval_relation":
			arith = true
		case "tr_sparse", "tr_hashtable", "tr_bitvector", "tr_lazy_sparse":
		default:
			return false
		}
	}
	return arith
}

// isArithSingleRepresentation: one arithmetic relation plugin kind.
func isArithSingleRepresentation(kinds []string) bool {
	return len(kinds) == 1 && (kinds[0] == "bound_relation" || kinds[0] == "interval_relation")
}

// isTableRelationRepresentation: table_relation plugin kinds only.
func isTableRelationRepresentation(kinds []string) bool {
	if len(kinds) == 0 {
		return false
	}
	for _, k := range kinds {
		switch k {
		case "tr_sparse", "tr_hashtable", "tr_bitvector", "tr_lazy_sparse":
		default:
			return false
		}
	}
	return true
}

// cacheDatalogRelationFavourite records context::default_relation()
// (datalog.default_relation, "pentagon" by default).
func (e *PropositionalEngine) cacheDatalogRelationFavourite() error {
	if e.datalogRelationFavourite != "" {
		return nil
	}
	name := "pentagon"
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return err
		}
		selected, err := view.Symbol("datalog.default_relation", symbol.Symbol{})
		if err == nil && selected.Value != 0 {
			name, _, err = e.manager.Symbols.BareString(selected)
		}
		if err = errors.Join(err, view.Close()); err != nil {
			return err
		}
	}
	e.datalogRelationFavourite = name
	return nil
}
