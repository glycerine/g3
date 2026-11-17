package fp

// Source: src/muz/rel/rel_context.cpp rel_context::rel_context's relation
// plugin registration (rm.register_plugin(alloc(udoc_plugin, rm)), ...) and
// relation_manager::register_relation_plugin_impl's favourite selection by
// context::default_relation(). The favourite relation plugin's relations
// are created by relation_manager::mk_empty_relation and run through the
// compiled instructions like table relations.
//
// Go creates every predicate's table at registration; once the relation
// favourite is in effect, the predicates' eagerly made tables are replaced
// by the favourite plugin's relations (keeping their facts), as
// applyDatalogDefaultFiniteTable does for the favourite table plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// registerRelContextPlugins is rel_context::rel_context's relation plugin
// registration, once per relation manager, in its order: bound_relation,
// interval_relation (karr_relation when context::karr()), udoc, then
// check_relation.
func (e *PropositionalEngine) registerRelContextPlugins() error {
	if e.relations.GetRelationPlugin("bound_relation") != nil {
		return nil
	}
	if err := e.relations.RegisterRelationPlugin(rel.NewBoundRelationPluginRel(e.relations)); err != nil {
		return err
	}
	if err := e.relations.RegisterRelationPlugin(rel.NewIntervalRelationPlugin(e.relations)); err != nil {
		return err
	}
	if karr, err := e.datalogKarrEnabled(); err != nil {
		return err
	} else if karr {
		manager := e.manager
		checkpoint := func() bool {
			canceled, err := manager.Canceled()
			return err == nil && !canceled
		}
		if err = e.relations.RegisterRelationPlugin(rel.NewKarrRelationPlugin(e.relations, checkpoint)); err != nil {
			return err
		}
	}
	plugin, err := rel.NewUDocPlugin(e.relations)
	if err != nil {
		return err
	}
	if err = e.relations.RegisterRelationPlugin(plugin); err != nil {
		return errors.Join(err, plugin.Close())
	}
	check, err := rel.NewCheckRelationPlugin(e.relations)
	if err != nil {
		return err
	}
	return e.relations.RegisterRelationPlugin(check)
}

// datalogRelationPlugins registers the relation plugins once
// (rel_context::rel_context) and records the default relation;
// rel_context::updt_params: with datalog.check_relation the default relation
// is check_relation and its base is get_relation_plugin(check_relation()),
// null when no relation plugin has that name (table_relation plugins are
// named tr_<table>).
func (e *PropositionalEngine) datalogRelationPlugins(checkBase string, checkEnabled bool) error {
	if err := e.registerRelContextPlugins(); err != nil {
		return err
	}
	if checkEnabled {
		check := e.relations.GetRelationPlugin("check_relation").(*rel.CheckRelationPlugin)
		check.SetPlugin(e.relations.RelationPluginByName(checkBase))
		return e.relations.SetDefaultRelation("check_relation")
	}
	return e.relations.SetDefaultRelation(e.datalogRelationFavourite)
}

// datalogRelationKind classifies a predicate's relation the way
// relation_manager::mk_empty_relation chooses it: "favourite" for a
// requested relation plugin or the favourite relation plugin handling the
// signature (the predicate gets mk_empty_relation's relation), "table" for
// a table_relation, "plugin" for a signature no table handles (get_relation
// creates it), "" when none applies here (external relations).
// favouriteHandles is the favourite's can_handle_signature; requested is
// the plugin a single set_predicate_representation kind names.
func (e *PropositionalEngine) datalogRelationKind(state *predicateState, favouriteHandles func([]ast.ASTHandle) (bool, error), requestedHandles func(string, []ast.ASTHandle) (bool, error)) (string, error) {
	if state == nil || state.checked != nil || state.sparse != nil {
		return "", nil
	}
	if state.table == nil {
		// get_relation: a signature no table handles (Int/Real columns,
		// possibly next to table columns) gets the requested plugin, the
		// favourite, the first relation plugin that handles it, or the
		// empty product relation. A requested relation plugin kind that
		// cannot handle the signature leaves the choice to the favourite
		// and the relation plugin list.
		if len(state.requestedRepresentations) == 0 || requestedRelationPluginKind(state.requestedRepresentations) != "" {
			if e.relationColumnSorts(state.sorts) {
				return "plugin", nil
			}
			// relation_manager::mk_empty_relation: the requested kind's
			// plugin, then the favourite, when it handles the signature
			// (udoc takes bit-vectors of any width).
			if name := requestedRelationPluginKind(state.requestedRepresentations); name != "" {
				if ok, err := requestedHandles(name, state.sorts); err != nil || ok {
					return map[bool]string{true: "plugin"}[ok], err
				}
			}
			if ok, err := favouriteHandles(state.sorts); err != nil || ok {
				return map[bool]string{true: "plugin"}[ok], err
			}
		}
		return "", nil
	}
	_, tableErr := state.table.Signature()
	table := tableErr == nil
	if name := requestedRelationPluginKind(state.requestedRepresentations); name != "" {
		ok, err := requestedHandles(name, state.sorts)
		if err != nil {
			return "", err
		}
		if ok {
			return "favourite", nil
		}
	} else if len(state.requestedRepresentations) == 1 && (state.explicitFiniteRepresentation || isTableRelationRepresentation(state.requestedRepresentations)) {
		// A requested table plugin (tr_*), set or inherited
		// (inherit_predicate_kind), that rejects the signature leaves the
		// choice to the favourite.
		if table && e.relations.RequestedFiniteKindHandles(state.sorts, state.requestedRepresentations[0]) {
			return "table", nil
		}
	} else if len(state.requestedRepresentations) != 0 {
		return "", nil
	}
	ok, err := favouriteHandles(state.sorts)
	if err != nil {
		return "", err
	}
	if ok {
		return "favourite", nil
	}
	if table {
		return "table", nil
	}
	return "", nil
}

// selectCompiledRelationFavourite reports whether the query runs on the
// relation plugins' relations through the compiled pipeline: every
// predicate's relation is the favourite's (udoc, check_relation over any
// base plugin), a table's, or what get_relation gives a signature with
// Int/Real columns (another relation plugin's relation or a product
// relation). default_table_checked keeps its evaluator.
func (e *PropositionalEngine) selectCompiledRelationFavourite() (_ bool, resultErr error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	if e.compiledRelationPlugins {
		// Already selected by an earlier query.
		return true, nil
	}
	if e.udocDatalog != nil {
		return false, nil
	}
	checkBase, checkEnabled, err := e.datalogCheckRelationBase()
	if err != nil {
		return false, err
	}
	docFavourite := !checkEnabled && e.datalogRelationFavourite == "doc"
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	probe, err := rel.NewUDocPlugin(e.relations)
	if err != nil {
		return false, err
	}
	defer probe.Close()
	// rel_context::updt_params: the checked plugin is found by name among
	// the relation plugins rel_context registered.
	var checkPlugin rel.RelationPlugin
	if checkEnabled {
		if err = e.registerRelContextPlugins(); err != nil {
			return false, err
		}
		checkPlugin = e.relations.RelationPluginByName(checkBase)
	}
	favouriteHandles := func(sorts []ast.ASTHandle) (bool, error) {
		switch {
		case checkEnabled:
			// check_relation_plugin::can_handle_signature: m_base handles it.
			if checkPlugin == nil {
				return false, nil
			}
			return checkPlugin.CanHandleSignature(sorts)
		case docFavourite:
			return probe.CanHandleSignature(sorts)
		}
		return false, nil
	}
	requestedHandles := func(name string, sorts []ast.ASTHandle) (bool, error) {
		if name == "doc" {
			return probe.CanHandleSignature(sorts)
		}
		// bound_relation and interval_relation handle Int/Real signatures
		// only; a predicate with a table only has none of those columns.
		return false, nil
	}
	kinds := make(map[ast.ASTHandle]string, len(e.predicateOrder))
	anyDoc := false
	for _, d := range e.predicateOrder {
		kind, err := e.datalogRelationKind(e.predicates[d], favouriteHandles, requestedHandles)
		if err != nil || kind == "" {
			return false, err
		}
		kinds[d] = kind
		state := e.predicates[d]
		anyDoc = anyDoc || kind == "favourite" && docFavourite || len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
	}
	if !checkEnabled && !docFavourite && !anyDoc {
		return false, nil
	}
	count, err := e.set.Len()
	if err != nil {
		return false, err
	}
	// relation_manager combines relations of different plugins (a
	// table_relation, a udoc relation, an arithmetic relation) through the
	// product and sieve relations. A rule variable of a sort no relation
	// handles has no total relation (mk_full_relation).
	// rules.get_rule(i) is a borrowed pointer: the owning copies RuleSet.Rule
	// returns are dropped again (the set still holds every rule).
	var borrowed []*base.Rule
	defer func() {
		for _, rule := range borrowed {
			resultErr = errors.Join(resultErr, rule.Close())
		}
	}()
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return false, err
		}
		borrowed = append(borrowed, rule)
		n, err := rule.TailSize()
		if err != nil {
			return false, err
		}
		head, err := rule.Head()
		if err != nil {
			return false, err
		}
		roots := []ast.ASTHandle{head}
		for j := 0; j < n; j++ {
			tail, _, err := rule.Tail(j)
			if err != nil {
				return false, err
			}
			roots = append(roots, tail)
		}
		for _, root := range roots {
			sorts, err := e.expressionVariableSorts(root)
			if err != nil {
				return false, err
			}
			for _, sort := range sorts {
				ok, err := probe.CanHandleSignature([]ast.ASTHandle{sort})
				if err != nil {
					return false, err
				}
				if !ok && !e.relationColumnSorts([]ast.ASTHandle{sort}) {
					return false, nil
				}
			}
		}
	}
	if err = e.datalogRelationPlugins(checkBase, checkEnabled); err != nil {
		return false, err
	}
	for _, d := range e.predicateOrder {
		if kinds[d] == "table" && e.predicates[d].explicitFiniteRepresentation {
			if err = e.relations.SetRequestedTableKind(d, e.predicates[d].requestedRepresentations[0]); err != nil {
				return false, err
			}
		}
		if kinds[d] == "plugin" {
			if name := requestedRelationPluginKind(e.predicates[d].requestedRepresentations); name != "" {
				// rel_context::set_predicate_representation: set_predicate_kind.
				if err = e.relations.SetRequestedRelationKind(d, e.relations.GetRelationPlugin(name)); err != nil {
					return false, err
				}
			}
			table, err := e.relations.ReserveFiniteRelation(d)
			if err != nil {
				return false, err
			}
			e.predicates[d].table = table
			continue
		}
		if kinds[d] != "favourite" {
			continue
		}
		state := e.predicates[d]
		if name := requestedRelationPluginKind(state.requestedRepresentations); name != "" {
			// rel_context::set_predicate_representation: set_predicate_kind.
			if err = e.relations.SetRequestedRelationKind(d, e.relations.GetRelationPlugin(name)); err != nil {
				return false, err
			}
		}
		if err = e.migrateToFavouriteRelation(d); err != nil {
			return false, err
		}
	}
	e.compiledRelationPlugins = true
	return true, nil
}

// migrateToFavouriteRelation replaces predicate d's eagerly created table
// by mk_empty_relation(sig, pred) of the favourite plugin, keeping its
// facts.
func (e *PropositionalEngine) migrateToFavouriteRelation(d ast.ASTHandle) error {
	state := e.predicates[d]
	if _, ok := rel.AsRelation(state.table); ok {
		return nil
	}
	facts, err := state.table.Facts()
	if err != nil {
		return err
	}
	var replacement rel.FiniteTable
	if len(facts) == 0 && e.relations.IsPlaceholderRelation(d) {
		// get_relation has not created the relation yet.
		replacement, err = e.relations.MkEmptyPlaceholderRelation(state.sorts, d)
	} else {
		replacement, err = e.relations.MkEmptyRelation(state.sorts, d)
	}
	if err != nil {
		return err
	}
	if _, ok := rel.AsRelation(replacement); !ok {
		return errors.Join(ErrPropositionalEngine, replacement.Close())
	}
	for _, fact := range facts {
		if _, err = replacement.Add(fact); err != nil {
			return errors.Join(err, replacement.Close())
		}
	}
	return e.replaceDatalogTable(d, replacement)
}

// requestedRelationPluginKind is the relation plugin named by a single
// set_predicate_representation kind among the plugins rel_context always
// registers (doc, bound_relation, interval_relation), or "".
func requestedRelationPluginKind(kinds []string) string {
	if len(kinds) != 1 {
		return ""
	}
	switch kinds[0] {
	case "doc", "bound_relation", "interval_relation":
		return kinds[0]
	}
	return ""
}

// selectCompiledRelationFavouriteForFacts is selectCompiledRelationFavourite
// for a fact added before the query.
func (e *PropositionalEngine) selectCompiledRelationFavouriteForFacts() (bool, error) {
	return e.selectCompiledRelationFavourite()
}
