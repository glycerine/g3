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
// (rel_context::rel_context) and records the default relation.
func (e *PropositionalEngine) datalogRelationPlugins(checkedDoc bool) error {
	if err := e.registerRelContextPlugins(); err != nil {
		return err
	}
	if checkedDoc {
		check := e.relations.GetRelationPlugin("check_relation").(*rel.CheckRelationPlugin)
		check.SetPlugin(e.relations.GetRelationPlugin("doc"))
		return e.relations.SetDefaultRelation("check_relation")
	}
	return e.relations.SetDefaultRelation(e.datalogRelationFavourite)
}

// datalogRelationKind classifies a predicate's relation the way
// relation_manager::mk_empty_relation chooses it: "doc" for udoc_plugin
// (requested, or the favourite handling the signature), "table" for a
// table_relation, "" when neither applies here (product relations,
// other plugins, external relations).
func (e *PropositionalEngine) datalogRelationKind(probe *rel.UDocPlugin, state *predicateState, docFavourite bool) (string, error) {
	if state == nil || state.external != nil || state.checked != nil || state.sparse != nil {
		return "", nil
	}
	if state.table == nil {
		// get_relation: a signature no table handles (Int/Real columns,
		// possibly next to table columns) gets the favourite, the first
		// relation plugin that handles it (bound_relation, interval), or
		// the empty product relation.
		// A requested relation plugin kind is tried first; one that
		// cannot handle the signature leaves the choice to the favourite
		// and the relation plugin list.
		if (len(state.requestedRepresentations) == 0 || requestedRelationPluginKind(state.requestedRepresentations) != "") && e.relationColumnSorts(state.sorts) {
			return "plugin", nil
		}
		return "", nil
	}
	udoc, err := probe.CanHandleSignature(state.sorts)
	if err != nil {
		return "", err
	}
	_, tableErr := state.table.Signature()
	table := tableErr == nil
	switch {
	case len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc":
		if udoc {
			return "doc", nil
		}
	case state.explicitFiniteRepresentation && len(state.requestedRepresentations) == 1:
		// A requested table plugin (tr_*) that rejects the signature
		// leaves the choice to the favourite.
		if table && e.relations.RequestedFiniteKindHandles(state.sorts, state.requestedRepresentations[0]) {
			return "table", nil
		}
	case len(state.requestedRepresentations) != 0:
		return "", nil
	}
	if docFavourite && udoc {
		return "doc", nil
	}
	if table {
		return "table", nil
	}
	return "", nil
}

// selectCompiledRelationFavourite reports whether the query runs on the
// relation plugins' relations through the compiled pipeline: every
// predicate's relation is udoc's, a table's, or what get_relation gives a
// signature with Int/Real columns (another relation plugin's relation or a
// product relation). Checkers other than check_relation(doc), explanations
// and default_table_checked keep their evaluators.
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
	checkName, checkEnabled, err := e.datalogCheckRelationName()
	if err != nil {
		return false, err
	}
	// rel_context::updt_params: a check_relation names the checked plugin
	// and check_relation becomes the default relation.
	checkedDoc := checkEnabled && checkName == "doc"
	if checkEnabled && !checkedDoc {
		return false, nil
	}
	docFavourite := checkedDoc || e.datalogRelationFavourite == "doc"
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return false, err
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	probe, err := rel.NewUDocPlugin(e.relations)
	if err != nil {
		return false, err
	}
	defer probe.Close()
	kinds := make(map[ast.ASTHandle]string, len(e.predicateOrder))
	anyDoc := false
	for _, d := range e.predicateOrder {
		kind, err := e.datalogRelationKind(probe, e.predicates[d], docFavourite)
		if err != nil || kind == "" {
			return false, err
		}
		kinds[d] = kind
		state := e.predicates[d]
		anyDoc = anyDoc || kind == "doc" || len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
	}
	if !docFavourite && !anyDoc {
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
	if err = e.datalogRelationPlugins(checkedDoc); err != nil {
		return false, err
	}
	udoc := e.relations.GetRelationPlugin("doc")
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
		if kinds[d] != "doc" {
			continue
		}
		state := e.predicates[d]
		if len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc" {
			// rel_context::set_predicate_representation: set_predicate_kind.
			if err = e.relations.SetRequestedRelationKind(d, udoc); err != nil {
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
	replacement, err := e.relations.MkEmptyRelation(state.sorts, d)
	if err != nil {
		return err
	}
	if _, ok := rel.AsRelation(replacement); !ok {
		return errors.Join(ErrPropositionalEngine, replacement.Close())
	}
	facts, err := state.table.Facts()
	if err != nil {
		return errors.Join(err, replacement.Close())
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
// for a fact added before the query. With datalog.generate_explanations the
// explanation evaluators of the later expression queries take the facts.
func (e *PropositionalEngine) selectCompiledRelationFavouriteForFacts() (bool, error) {
	if enabled, err := e.datalogExplanationsRequested(); err != nil || enabled {
		return false, err
	}
	return e.selectCompiledRelationFavourite()
}

// datalogExplanationsRequested reports datalog.generate_explanations for the
// engine itself as well as its query clones.
func (e *PropositionalEngine) datalogExplanationsRequested() (bool, error) {
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
